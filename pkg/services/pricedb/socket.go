// Copyright (c) 2026 Lemon4ksan All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pricedb

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/lemon4ksan/aoni"
	"github.com/lemon4ksan/aoni/mod"
	"github.com/lemon4ksan/aoni/x/realtime/ws"
	log "github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/foundation/codec/json"
	"github.com/lemon4ksan/foundation/generic"
)

const (
	defaultInitialBackoff      = 1 * time.Second
	defaultMaxBackoff          = 30 * time.Second
	defaultMaxProtocolAttempts = 3
)

// SocketManager handles the real-time price updates via Socket.IO.
type SocketManager struct {
	r         aoni.WebSocketDialer
	url       string
	urlTLS    string
	preferTLS bool
	logger    log.Logger
	userAgent string

	initialBackoff      time.Duration
	maxBackoff          time.Duration
	maxProtocolAttempts int

	mu   sync.Mutex
	conn ws.Conn

	onPrice func(price *Price)
}

// NewSocketManager creates a new Socket.IO client for PriceDB.
func NewSocketManager(rawURL string, r aoni.WebSocketDialer, logger log.Logger) *SocketManager {
	if r == nil {
		r = aoni.DefaultClient
	}

	target := generic.Coalesce(rawURL, "ws://ws.pricedb.io/")
	wsURL, wssURL, preferTLS := deriveProtocolURLs(target)

	return &SocketManager{
		r:                   r,
		url:                 wsURL,
		urlTLS:              wssURL,
		preferTLS:           preferTLS,
		initialBackoff:      defaultInitialBackoff,
		maxBackoff:          defaultMaxBackoff,
		maxProtocolAttempts: defaultMaxProtocolAttempts,
		logger:              logger.With(log.Module("pricedb_socket")),
	}
}

func deriveProtocolURLs(rawURL string) (wsURL, wssURL string, preferTLS bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, rawURL, false
	}

	preferTLS = u.Scheme == "wss" || u.Scheme == "https"

	uNonTLS := *u
	switch uNonTLS.Scheme {
	case "https", "wss", "http":
		uNonTLS.Scheme = "ws"
	}

	uTLS := *u
	switch uTLS.Scheme {
	case "http", "ws", "https":
		uTLS.Scheme = "wss"
	}

	return uNonTLS.String(), uTLS.String(), preferTLS
}

// OnPrice sets the callback for when a price update is received.
func (s *SocketManager) OnPrice(fn func(price *Price)) {
	s.onPrice = fn
}

// Run starts the socket connection and maintains it with exponential backoff, jitter, and TLS fallback.
func (s *SocketManager) Run(ctx context.Context) error {
	attempt := 0
	currentPreferTLS := s.preferTLS

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		targetURL := s.url
		if currentPreferTLS {
			targetURL = s.urlTLS
		}

		err := s.connectAndListenURL(ctx, targetURL)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		attempt++
		if attempt >= s.maxProtocolAttempts {
			currentPreferTLS = !currentPreferTLS
			attempt = 0

			s.logger.Warn("Max reconnect attempts reached on current protocol, switching endpoint",
				log.Bool("preferTLS", currentPreferTLS),
				log.String("nextEndpoint", generic.Ternary(currentPreferTLS, s.urlTLS, s.url)),
				log.Err(err),
			)
		} else if err != nil {
			s.logger.Warn("Socket.IO connection failed, retrying...",
				log.Int("attempt", attempt),
				log.Bool("preferTLS", currentPreferTLS),
				log.Err(err),
			)
		}

		delay := s.calculateBackoff(attempt)
		timer := time.NewTimer(delay)

		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *SocketManager) calculateBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		return s.initialBackoff
	}

	mult := 1 << min(attempt, 6)
	backoff := s.initialBackoff * time.Duration(mult)

	if backoff > s.maxBackoff {
		backoff = s.maxBackoff
	}

	// Full jitter between backoff/2 and backoff
	half := int64(backoff / 2)
	if half <= 0 {
		return backoff
	}

	jitter := time.Duration(rand.Int64N(half))

	return time.Duration(half) + jitter
}

func (s *SocketManager) connectAndListen(ctx context.Context) error {
	target := s.url
	if s.preferTLS {
		target = s.urlTLS
	}

	return s.connectAndListenURL(ctx, target)
}

func (s *SocketManager) connectAndListenURL(ctx context.Context, endpointURL string) error {
	u, err := url.Parse(endpointURL)
	if err != nil {
		return err
	}

	// Socket.IO v4 path and params
	u.Path = "/socket.io/"
	q := u.Query()
	q.Set("EIO", "4")
	q.Set("transport", "websocket")
	u.RawQuery = q.Encode()

	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	}

	s.logger.Debug("Connecting to PriceDB Socket.IO...", log.String("url", u.String()))

	var mods []aoni.RequestModifier
	if s.userAgent != "" {
		mods = append(mods, mod.WithUserAgent(s.userAgent))
	}

	wsConn, resp, err := ws.DialWebSocket(ctx, s.r, u.String(), mods...)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	if err != nil {
		return err
	}

	s.mu.Lock()
	s.conn = wsConn
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		if s.conn != nil {
			_ = s.conn.Close()
			s.conn = nil
		}

		s.mu.Unlock()
	}()

	// Socket.IO Handshake sequence
	// 1. Wait for Engine.IO "open" packet (0)
	_, p, err := wsConn.ReadMessage()
	if err != nil {
		return err
	}

	if !strings.HasPrefix(string(p), "0") {
		return fmt.Errorf("unexpected handshake packet: %s", string(p))
	}

	// 2. Send Socket.IO "connect" packet (40)
	if err := wsConn.WriteMessage(ws.FrameText, []byte("40")); err != nil {
		return err
	}

	// 3. Main listen loop
	for {
		_, p, err := wsConn.ReadMessage()
		if err != nil {
			return err
		}

		packet := string(p)
		if len(packet) == 0 {
			continue
		}

		switch packet[0] {
		case '2': // Engine.IO Ping
			// Respond with Pong (3)
			if err := wsConn.WriteMessage(ws.FrameText, []byte("3")); err != nil {
				return err
			}

		case '4': // Socket.IO Packet
			if len(packet) < 2 {
				continue
			}

			if packet[1] == '2' { // Event
				s.handleEvent(packet[2:])
			}
		}
	}
}

func (s *SocketManager) handleEvent(payload string) {
	// Socket.IO event format: ["event_name", data]
	var raw []json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		return
	}

	if len(raw) < 2 {
		return
	}

	var eventName string
	if err := json.Unmarshal(raw[0], &eventName); err != nil {
		return
	}

	if eventName != "price" {
		return
	}

	var price Price
	if err := json.Unmarshal(raw[1], &price); err != nil {
		s.logger.Warn("Failed to unmarshal price update from socket", log.Err(err))
		return
	}

	if s.onPrice != nil {
		s.onPrice(&price)
	}
}
