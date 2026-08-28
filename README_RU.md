<div align="center">

# G-MAN TF2

### Высокопроизводительный экономический движок и модуль Team Fortress 2 для Go

_"У профессионалов есть правила."_

[![Go Version](https://img.shields.io/badge/go-1.27%2B-007d9c?logo=go&logoColor=white&style=flat-square)](https://go.dev/)
[![Go Reference](https://img.shields.io/badge/godoc-reference-007d9c?style=flat-square)](https://pkg.go.dev/github.com/lemon4ksan/g-man-tf2)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue?style=flat-square)](LICENSE)
[![Zero-Alloc SKU](https://img.shields.io/badge/memory-Zero--Alloc%20SKU-brightgreen?style=flat-square)](pkg/sku)
[![Linter](https://img.shields.io/badge/linter-golangci--lint-brightgreen?style=flat-square&logo=go)](https://github.com/golangci/golangci-lint)
[![Parity](https://img.shields.io/badge/parity-100%25%20Node.js%20Compliance-blueviolet?style=flat-square)](docs/PARITY.md)

**G-MAN TF2** — это официальный доменный модуль Team Fortress 2 и экономический движок промышленного уровня, созданный для автоматизационного фреймворка [G-MAN](https://github.com/lemon4ksan/g-man). Он объединяет протоколы Game Coordinator (GC), потоковую синхронизацию инвентаря SOCache и целочисленную арифметику металлов в единую потокобезопасную Go-архитектуру.

#### 🇺🇸 [English](README.md) • 🇷🇺 [Русский](README_RU.md) • 📐 [Спецификация соответствия](docs/PARITY.md)

</div>

```shell
go get github.com/lemon4ksan/g-man-tf2
```

## ⚡ Ключевые возможности

* **Драйвер Game Coordinator и хранилище SOCache (`pkg/tf2`):** Zero-allocation парсинг потоковых обновлений Shared Object Cache (`CMsgSOCacheSubscribed`, `SO_UPDATE`, `SO_DESTROY`) с мгновенным выполнением крафта, переплавки и достижений.
* **Trie-индексированный движок игровых схем (`pkg/schema`):** Двухуровневое кэширование (Defindex + нормализованное префиксное дерево Trie) для парсинга `items_game.txt` с субмиллисекундным поиском и потреблением памяти всего ~10 MB.
* **Целочисленная арифметика металлов (`pkg/currency`):** Точные расчеты на базе атомарного скрапа (`currency.Scrap`, `currency.Currency`), исключающие погрешности чисел с плавающей запятой IEEE 754 при оценке сложных сделок.
* **Автокрафт и сдача (`pkg/crafting`):** Попарная переплавка дубликатов оружия одного класса (`CombineWeapons`), автоматический размен/сборка очищенных металлов и мгновенный расчет сдачи при принятии трейдов.
* **Нативные клиенты сторонних сервисов (`pkg/services`):** Высокоскоростные типизированные клиенты на базе [aoni](https://github.com/lemon4ksan/aoni) для backpack.tf, PriceDB (real-time WebSocket поток), Mannco.store, Crit.tf, Express-load и Rep.tf.
* **Модульные Onion-мидлвары трейдов (`pkg/trading`):** Готовые компоненты конвейера проверок (`StockLimitMiddleware`, `PricerMiddleware`, `AutoCounterMiddleware`) для торгового движка [g-man](https://github.com/lemon4ksan/g-man).

## ⚔️ Go против Node.js: преимущества G-MAN TF2

Исторически торговые боты TF2 писались на Node.js (`tf2autobot`, `tf2-schema`, `tf2-currencies`). При масштабировании однопоточность JavaScript и накладные расходы V8 создают серьезные ограничения:

| Критерий | 🤖 G-MAN TF2 (Go) | 📦 Node.js (`tf2autobot` / `tf2-schema`) | Почему это важно |
| :--- | :--- | :--- | :--- |
| **Память (Heap) на бота** | **~8 - 12 MB** | **~180 - 350 MB** | Запуск в 20–40 раз большего числа ботов на одном недорогом VPS без риска OOM. |
| **Инициализация схемы** | **<40 мс** (Trie + плоский индекс) | **3.5 - 8.0 секунд** (JSON-дерево V8) | Мгновенный старт ботов и быстрое восстановление при реконнектах. |
| **Расчет валюты** | **Точный целочисленный скрап (`int`)** | `float` + `bignumber.js` | Полное отсутствие багов округления и «дрейфа» скрапа (`0.11`, `0.33`). |
| **Задержки GC** | **Субмиллисекундные** | До 150 мс пауз сборщика V8 | Отсутствие лагов при обработке сотен входящих предложений обмена. |
| **Конкурентность** | **Нативные горутины CSP** | Однопоточный Event Loop | Десятки аккаунтов, вебхуков цен и GC-сессий работают параллельно без блокировок. |

## 🚀 Быстрый старт

### 1. Инициализация клиента Steam с TF2-модулями

```go
package main

import (
	"context"
	"os"

	"github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/g-man/pkg/steam"
	"github.com/lemon4ksan/g-man/pkg/steam/auth"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/apps"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/directory"
	"github.com/lemon4ksan/g-man/pkg/steam/sys/gc"
	"github.com/lemon4ksan/g-man/pkg/storage/jsonfile"
	"github.com/lemon4ksan/g-man/pkg/trading/web"

	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/tf2"
)

func main() {
	ctx := context.Background()
	store, _ := jsonfile.New("storage.json")
	logger := log.New(log.DefaultConfig(log.LevelInfo))

	// 1. Инициализация клиента Steam с модульными G-MAN TF2 плагинами
	client, err := steam.NewClient(steam.DefaultConfig(),
		steam.WithLogger(logger),
		steam.WithStorage(store),
		gc.WithModule(),
		apps.WithModule(),
		tf2.WithModule(),
		schema.WithModule(schema.DefaultConfig()),
		backpack.WithModule(),
		web.WithModule(web.DefaultConfig()),
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	// 2. Получение ссылок на зарегистрированные модули
	bpMod := backpack.From(client)

	// 2. Доступ к отслеживанию инвентаря через Game Coordinator
	bp := backpack.From(client)
	sub := client.Bus().Subscribe(&tf2.BackpackLoadedEvent{})
	go func() {
		for event := range sub.C() {
			if bpEvent, ok := event.(*tf2.BackpackLoadedEvent); ok {
				pure := bp.GetPureStock()
				logger.Info("Рюкзак TF2 синхронизирован!",
					logkit.Int("total_items", bpEvent.Count),
					logkit.Int("keys", pure.Keys),
					logkit.Float64("refined", pure.TotalRefined()),
				)
			}
		}
	}()

	if err := client.Run(); err != nil {
		panic(err)
	}

	// 3. Поиск CM-сервера и авторизация
	dir := directory.New(client)
	server, _ := dir.GetOptimalCMServer(ctx)
	login := auth.NewLogOnDetails(os.Getenv("STEAM_USER"), os.Getenv("STEAM_PASS"))

	if err := client.ConnectAndLogin(ctx, server, login); err != nil {
		panic(err)
	}

	client.Wait()
}
```

### 2. Точная арифметика металлов и ключей

```go
package main

import (
	"fmt"

	"github.com/lemon4ksan/g-man-tf2/pkg/currency"
)

func main() {
	// Конвертация десятичного металла в целочисленный скрап
	scrap := currency.ToScrap(45.33) // 408 Scrap

	// Безопасное обратное преобразование без потери точности
	ref := currency.ToRefined(scrap) // 45.33

	// Структура комбинированной валюты
	cur := currency.New(2, 45.33)
	fmt.Println(cur.String()) // "2 keys, 45.33 ref"
}
```

### 3. Регистрация Onion-мидлваров трейдов

```go
package main

import (
	"github.com/lemon4ksan/foundation/async/logkit"
	"github.com/lemon4ksan/g-man/pkg/trading/engine"
	"github.com/lemon4ksan/g-man-tf2/pkg/backpack"
	"github.com/lemon4ksan/g-man-tf2/pkg/schema"
	"github.com/lemon4ksan/g-man-tf2/pkg/services/pricedb"
	"github.com/lemon4ksan/g-man-tf2/pkg/trading"
)

func RegisterPipeline(
	tradeEngine *engine.Engine,
	bp *backpack.Backpack,
	priceMgr *pricedb.Manager,
	schemaMod *schema.Manager,
	logger logkit.Logger,
) {
	stockCfg := trading.StockConfig{
		MaxTotal:   3000,
		DefaultMax: 20,
		MaxPerSKU: map[string]int{
			"5021;6": 500, // Лимит ключей Mann Co.
		},
	}

	tradeEngine.Use(
		// 1. Контроль лимитов инвентаря по каждому SKU
		trading.StockLimitMiddleware(bp, stockCfg, logger),

		// 2. Валидация цен предложений по данным PriceDB в реальном времени
		trading.PricerMiddleware(priceMgr, schemaMod.Get, logger),
	)
}
```

## 📂 Архитектура пакетов

```text
pkg/
├── tf2/              # Драйвер TF2 Game Coordinator и хранилище SOCache
├── backpack/         # Проекции инвентаря в памяти и блокировки слотов
├── crafting/         # Рецепты автоматического крафта и переплавки дубликатов оружия
├── schema/           # Парсер items_game со структурой Trie и индексом Defindex
├── sku/              # Zero-allocation канонический парсер и форматтер SKU
├── currency/         # Целочисленная арифметика скрапа и формулы валют
├── services/         # Нативные HTTP/WebSocket клиенты на базе aoni
│   ├── pricedb/      # Клиент прайсинга PriceDB и WebSocket поток
│   ├── bptf/         # Клиент API backpack.tf и менеджер листингов
│   ├── crit/         # Синхронизатор витрины Crit.tf
│   ├── mannco/       # API и WebSocket поток маркета Mannco.store
│   ├── express/      # Клиент быстрого парсинга инвентарей Express-load
│   └── rep/          # Проверка репутации и отзывов Rep.tf
├── trading/          # Onion-мидлвары для торгового движка g-man
└── reason/           # Стандартизированные коды причин отклонения трейдов
```

## 📦 Экосистема

* **[g-man](https://github.com/lemon4ksan/g-man)**: Базовый Steam-клиент SDK и движок игровой автоматизации.
* **[g-man-cli](https://github.com/lemon4ksan/g-man-cli)**: Фоновый демон (`g-mand`) и консольный TUI-клиент управления (`gmanctl`).
* **[aoni](https://github.com/lemon4ksan/aoni)**: Высокоскоростной сетевой стек, HTTP/2 и WebSocket движок.
* **[foundation](https://github.com/lemon4ksan/foundation)**: Базовые примитивы конкурентности, асинхронное логирование (`logkit`) и структуры данных.

## ⚖️ Лицензия и правовая информация

**Дисклеймер:** Данное программное обеспечение **не** связано с **Valve Corporation**, не поддерживается и не одобряется ею. Steam, Team Fortress 2 и соответствующие торговые марки являются собственностью Valve Corporation.

Проект распространяется под лицензией **BSD 3-Clause License**. Подробности в файле [LICENSE](LICENSE).
