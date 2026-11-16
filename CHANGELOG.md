## [Unreleased]

## [v1.7.1] - 2026-11-16

### CI
- Made govulncheck non-blocking while the minimum Go version stays 1.23.
- Ran golangci-lint only on Go 1.23 to avoid export data incompatibility on Go 1.26/1.27.

## [v1.7.0] - 2026-11-16

### Added
- Added `DEFAULT_MIN_IDLE`, `DEFAULT_MAX_ACTIVE` and `CONNECT_TIMEOUT` pool constants.
- Added `common.BindListener` and `common.NotifyStarted` server start helpers.

### Changed
- Reused HTTP connections through a cached client with a cloned default transport.
- Generated globally unique request IDs with an atomic counter.
- Released pooled connections without dialing while holding the lock.
- Server tests now bind port 0 and use the OS-assigned port.

### Fixed
- Guarded the nacos heartbeat list against concurrent access.
- Prevented panics on malformed JSON-RPC requests and empty batches.
- Fixed connection pool deadlocks and stale connection retries.

### CI
- Pinned the minimum Go version to 1.23 and expanded the test matrix to 1.23-1.27.

## [v1.6.8] - 2026-01-11

### Added
- Added wrk benchmark test.


## [v1.6.7] - 2025-09-24

### Added
- Added https support.

---

## [v1.6.6] - 2025-05-28

### Modified
- Removed mapstructure.

---

## [v1.6.5] - 2023-03-06

### Added
- Added service registration & discovery support.
- Added nacos & consul support.

### Changed
- Mapping one client to one server struct.