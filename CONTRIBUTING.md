# Contributing to G-MAN TF2

Thank you for your interest in contributing to **G-MAN TF2**! By contributing, you help make the most high-performance, resilient, and enterprise-grade Team Fortress 2 domain SDK for Go even better.

Please take a moment to review these guidelines to ensure a smooth, efficient, and professional development cycle.

## 🎨 Our Development Philosophy

G-MAN TF2 is architected for industrial-scale deployment. Every line of code added to this repository must align with our three core pillars:

1. **Performance & Efficiency**: We prioritize CPU and memory efficiency. Our schema indexing is O(1) and runs in minimal memory. Avoid redundant collections, keep GC allocations to a minimum, and leverage unified pointer caches.
2. **Thread Safety**: Your code will run in high-concurrency environments with multiple Goroutines reading/writing states. Always employ robust concurrency primitives (`sync.Mutex`, `sync.RWMutex`, or preferably `sync/atomic`).
3. **Loose Coupling**: All TF2 sub-packages (e.g., `backpack`, `bptf`, `pricedb`, `schema`) must remain decoupled. Modules should communicate asynchronously using the unified **Event Bus** instead of direct hard-coupling.

## 🛠 Getting Started

### Setup
1. Fork the repository and clone it locally:
   ```shell
   git clone https://github.com/your-username/g-man-tf2.git
   cd g-man-tf2
   ```

2. Download and verify Go dependencies:
   ```shell
   go mod download
   go mod verify
   ```

3. Run the complete test suite to ensure everything compiles and passes out of the box:
   ```shell
   go test -v -race ./...
   ```

## 📐 Coding Standards & Guidelines

To maintain code health and reliability, please adhere to the following standards:

### 1. Item Identity & Identifiers
- **Always use `sku` for item lookups**: Do not identify items solely by `defindex` or name, as qualities, effects, killstreak stages, and paints radically modify an item's SKU and market value.
- Use `github.com/lemon4ksan/g-man-tf2/pkg/sku` package primitives to parse or stringify item configurations.

### 2. State & Caching
- **Single Source of Truth**: The `SOCache` (`tf2.Client.Cache()`) is the true state of the inventory synced via GC socket packets. Do not store duplicate or stale slices of items in custom modules. Query the `Backpack` module directly.
- **Zero-Allocation Tracking**: Use pointer mappings to existing items instead of copying large structures.

### 3. Defensive Programming
- Parse network packets safely. When dealing with binary structures (such as in `socache.go` or `crafting`), validate offsets and payload lengths before reading slices or indices to prevent runtime panic errors.

### 4. Code Quality
- Format all code with `go fmt`:
  ```shell
  go fmt ./...
  ```
- Run `go vet` to catch common bugs before committing:
  ```shell
  go vet ./...
  ```
- We encourage running `golangci-lint` to maintain stylistic consistency across all files.

## 🧪 Testing Guidelines

No feature or bug fix will be merged without accompanying tests.

- **Concurreny Safety**: Always run your tests with the race detector enabled (`go test -race ./...`).
- **Mocking Networks**: Network actions (WebAPI requests or GC messages) should be mocked using interfaces such as `CoordinatorProvider` or standard HTTP test servers. Do not rely on active, live internet connections inside tests.
- **Table-Driven Tests**: Use table-driven test patterns in Go for parsing logic, SKU calculations, or currency mathematics.

## 📦 Pull Request Protocol

When you are ready to submit your changes:

1. **Branch Naming**: Use clean, descriptive branch names:
   - `feature/add-cs2-coordinator`
   - `bugfix/fix-smelting-calculation`
   - `docs/update-contributing`
2. **Commit Messages**: Keep commit messages concise and informative:
   - `feat(currency): add support for keys-to-refined exchange mapping`
   - `fix(socache): resolve memory allocation leak on SO updates`
3. **Open a PR**:
   - Provide a detailed description of what the PR changes, the problem it solves, and how it was tested.
   - Link any related issues or discussions.
4. **Review Process**:
   - A maintainer will review your code.
   - Address any requested changes promptly. Once approved, your branch will be squash-merged into `main`.

## ☕ Questions and Support

If you need architectural advice or have questions about how a particular Game Coordinator packet is structured, please open a **GitHub Issue** or contact the G-MAN project maintainers.

We appreciate your time, effort, and commitment to making the Steam trading bot ecosystem highly performant and secure!
