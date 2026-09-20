// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lemon4ksan/aoni/realtime/ws"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testUpgradeToWS(w http.ResponseWriter, r *http.Request) (ws.Conn, error) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack not supported", http.StatusInternalServerError)
		return nil, errors.New("hijack failed")
	}

	conn, bufrw, err := hj.Hijack()
	if err != nil {
		return nil, err
	}

	key := r.Header.Get("Sec-WebSocket-Key")
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	acceptKey := base64.StdEncoding.EncodeToString(h.Sum(nil))

	resp := "HTTP/1.1 101 Switching Protocols\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + acceptKey + "\r\n\r\n"

	if _, err := bufrw.WriteString(resp); err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := bufrw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}

	return ws.WrapRawConnWithReader(conn, bufrw.Reader, false, 4096), nil
}

// Helper mock logger to capture warning messages
type mockLogRecorder struct {
	log.Logger
	messages []string
}

func (m *mockLogRecorder) Warn(msg string, fields ...any) {
	m.messages = append(m.messages, msg)
}

func (m *mockLogRecorder) With(fields ...any) log.Logger {
	return m
}

func TestSocketManager_New_NilClient(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager("", nil, log.Discard)
	assert.NotNil(t, sm.r)
	assert.Equal(t, "ws://ws.pricedb.io/", sm.url)
}

func TestSocketManager_Run_Cancellation(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager("ws://127.0.0.1:49151", nil, log.Discard)

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	err := sm.Run(ctx)
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSocketManager_DialFail(t *testing.T) {
	t.Parallel()

	// Using a closed/unavailable port to trigger dial failure
	sm := NewSocketManager("ws://127.0.0.1:49151/invalid", nil, log.Discard)
	err := sm.connectAndListen(t.Context())
	assert.Error(t, err)
}

func TestSocketManager_PingPong(t *testing.T) {
	t.Parallel()

	pongReceived := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgradeToWS(w, r)
		if err != nil {
			return
		}
		defer conn.Close()

		// 1. Send Engine.IO Open Packet "0"
		if err := conn.WriteMessage(ws.OpcodeText, []byte(`0{"sid":"123"}`)); err != nil {
			return
		}

		// 2. Read connection packet "40"
		_, p, err := conn.ReadMessage()
		if err != nil || string(p) != "40" {
			return
		}

		// 3. Send Ping "2"
		if err := conn.WriteMessage(ws.OpcodeText, []byte("2")); err != nil {
			return
		}

		// 4. Read Pong "3"
		_, p, err = conn.ReadMessage()
		if err == nil && string(p) == "3" {
			close(pongReceived)
		}
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	sm := NewSocketManager(wsURL, nil, log.Discard)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	go func() {
		_ = sm.connectAndListen(ctx)
	}()

	select {
	case <-pongReceived:
		// Success
	case <-ctx.Done():
		t.Fatal("timed out waiting for pong response")
	}
}

func TestSocketManager_HandleEvent_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("invalid_json", func(t *testing.T) {
		sm := NewSocketManager("", nil, log.Discard)
		assert.NotPanics(t, func() {
			sm.handleEvent("{invalid_json")
		})
	})

	t.Run("insufficient_elements", func(t *testing.T) {
		sm := NewSocketManager("", nil, log.Discard)
		assert.NotPanics(t, func() {
			sm.handleEvent(`["price"]`)
		})
	})

	t.Run("invalid_event_name_type", func(t *testing.T) {
		sm := NewSocketManager("", nil, log.Discard)
		assert.NotPanics(t, func() {
			sm.handleEvent(`[123, {}]`)
		})
	})

	t.Run("different_event_name", func(t *testing.T) {
		sm := NewSocketManager("ws://localhost", nil, log.Discard)
		called := false
		sm.OnPrice(func(p *Price) {
			called = true
		})
		sm.handleEvent(`["other_event", {"sku":"1;6"}]`)
		assert.False(t, called)
	})

	t.Run("invalid_price_payload", func(t *testing.T) {
		logger := &mockLogRecorder{}
		sm := NewSocketManager("", nil, logger)

		sm.handleEvent(`["price", "not_an_object"]`)

		require.NotEmpty(t, logger.messages)
		assert.Contains(t, logger.messages[0], "Failed to unmarshal price update from socket")
	})

	t.Run("nil_callback_no_panic", func(t *testing.T) {
		sm := NewSocketManager("", nil, log.Discard)
		sm.OnPrice(nil)
		assert.NotPanics(t, func() {
			sm.handleEvent(`["price", {"sku":"5021;6","buy":{"metal":10},"sell":{"metal":12}}]`)
		})
	})
}

func TestSocketManager_PacketParsing_EdgeCases(t *testing.T) {
	t.Parallel()

	sequenceDone := make(chan struct{})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgradeToWS(w, r)
		if err != nil {
			return
		}
		defer conn.Close()

		if err := conn.WriteMessage(ws.OpcodeText, []byte(`0{"sid":"123"}`)); err != nil {
			return
		}

		_, _, _ = conn.ReadMessage() // Read 40

		// Send empty packet (should be skipped)
		_ = conn.WriteMessage(ws.OpcodeText, []byte(""))

		// Send short Socket.IO packet "4" (should be skipped since len < 2)
		_ = conn.WriteMessage(ws.OpcodeText, []byte("4"))

		// Send non-event Socket.IO packet "43" (should be skipped since second char is not '2')
		_ = conn.WriteMessage(ws.OpcodeText, []byte("43"))

		// Finally send valid event "42"
		priceUpdatePayload := `42["price",{"sku":"5021;6","name":"Key","buy":{"metal":82},"sell":{"metal":82.11}}]`
		_ = conn.WriteMessage(ws.OpcodeText, []byte(priceUpdatePayload))

		close(sequenceDone)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	sm := NewSocketManager(wsURL, nil, log.Discard)

	priceReceived := make(chan struct{})

	var once sync.Once
	sm.OnPrice(func(p *Price) {
		if p.SKU == "5021;6" {
			once.Do(func() {
				close(priceReceived)
			})
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	go func() {
		_ = sm.connectAndListen(ctx)
	}()

	select {
	case <-priceReceived:
		// Succeeded

	case <-ctx.Done():
		t.Fatal("timed out waiting for price update event")
	}
}

func TestSocketManager_URLErr(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager(":%:invalid", nil, log.Discard)
	err := sm.connectAndListen(t.Context())
	assert.Error(t, err)
}

func TestSocketManager_HandshakeErr(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := testUpgradeToWS(w, r)
		if err == nil {
			_ = conn.WriteMessage(ws.OpcodeText, []byte(`invalid_handshake`))
			conn.Close()
		}
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	sm := NewSocketManager(wsURL, nil, log.Discard)
	err := sm.connectAndListen(t.Context())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected handshake packet")
}

func TestSocketManager_BackoffCalculation(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager("", nil, log.Discard)
	assert.Equal(t, 1*time.Second, sm.calculateBackoff(0))

	for i := 0; i < 20; i++ {
		b1 := sm.calculateBackoff(1)
		assert.GreaterOrEqual(t, b1, 1*time.Second)
		assert.LessOrEqual(t, b1, 2*time.Second)

		b2 := sm.calculateBackoff(2)
		assert.GreaterOrEqual(t, b2, 2*time.Second)
		assert.LessOrEqual(t, b2, 4*time.Second)

		b5 := sm.calculateBackoff(5)
		assert.GreaterOrEqual(t, b5, 15*time.Second)
		assert.LessOrEqual(t, b5, 30*time.Second)

		b10 := sm.calculateBackoff(10)
		assert.LessOrEqual(t, b10, 30*time.Second)
	}
}

func TestSocketManager_DeriveProtocolURLs(t *testing.T) {
	t.Parallel()

	wsURL, wssURL, preferTLS := deriveProtocolURLs("ws://ws.pricedb.io/")
	assert.Equal(t, "ws://ws.pricedb.io/", wsURL)
	assert.Equal(t, "wss://ws.pricedb.io/", wssURL)
	assert.False(t, preferTLS)

	wsURL2, wssURL2, preferTLS2 := deriveProtocolURLs("wss://ws.pricedb.io/")
	assert.Equal(t, "ws://ws.pricedb.io/", wsURL2)
	assert.Equal(t, "wss://ws.pricedb.io/", wssURL2)
	assert.True(t, preferTLS2)

	wsURL3, wssURL3, preferTLS3 := deriveProtocolURLs("http://example.com/socket")
	assert.Equal(t, "ws://example.com/socket", wsURL3)
	assert.Equal(t, "wss://example.com/socket", wssURL3)
	assert.False(t, preferTLS3)
}

func TestSocketManager_ContextCancellationInterruptsBackoff(t *testing.T) {
	t.Parallel()

	sm := NewSocketManager("ws://127.0.0.1:49151", nil, log.Discard)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- sm.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()

	select {
	case err := <-done:
		elapsed := time.Since(start)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Less(t, elapsed, 200*time.Millisecond, "Run must exit promptly on context cancel")
	case <-time.After(1 * time.Second):
		t.Fatal("Run hung in backoff sleep after context was cancelled")
	}
}

type mockFallbackDialer struct {
	mu          sync.Mutex
	schemes     []string
	targetAddrs []string
}

func (m *mockFallbackDialer) DialPlainForWS(ctx context.Context, addr string) (net.Conn, error) {
	m.mu.Lock()
	m.schemes = append(m.schemes, "ws")
	m.targetAddrs = append(m.targetAddrs, addr)
	m.mu.Unlock()
	return nil, errors.New("mock plain dial failed")
}

func (m *mockFallbackDialer) DialTLSForWS(ctx context.Context, addr string) (net.Conn, error) {
	m.mu.Lock()
	m.schemes = append(m.schemes, "wss")
	m.targetAddrs = append(m.targetAddrs, addr)
	m.mu.Unlock()
	return nil, errors.New("mock tls dial failed")
}

func TestSocketManager_ResponseClosedOnDialError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
	}))
	defer server.Close()

	wsURL := strings.Replace(server.URL, "http://", "ws://", 1)
	sm := NewSocketManager(wsURL, nil, log.Discard)
	err := sm.connectAndListen(context.Background())
	assert.Error(t, err)
}

func TestSocketManager_ProtocolFallback_WStoWSS(t *testing.T) {
	t.Parallel()

	dialer := &mockFallbackDialer{}

	sm := NewSocketManager("ws://test.pricedb.io/", dialer, log.Discard)
	sm.initialBackoff = 5 * time.Millisecond
	sm.maxBackoff = 10 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			dialer.mu.Lock()
			count := len(dialer.schemes)
			dialer.mu.Unlock()

			if count >= 4 {
				cancel()
				return
			}
			time.Sleep(2 * time.Millisecond)
		}
	}()

	_ = sm.Run(ctx)

	dialer.mu.Lock()
	defer dialer.mu.Unlock()

	require.GreaterOrEqual(t, len(dialer.schemes), 4)
	assert.Equal(t, "ws", dialer.schemes[0])
	assert.Equal(t, "ws", dialer.schemes[1])
	assert.Equal(t, "ws", dialer.schemes[2])
	assert.Equal(t, "wss", dialer.schemes[3], "4th attempt must fall back to wss (DialTLSForWS)")
}

