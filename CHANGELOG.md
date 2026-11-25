## [Unreleased]

### Changed
- **Breaking**: switched to the JS-style RPC API. Server methods now take
  value arguments and return the result directly (optionally followed by an
  error), e.g. `func (i *IntRpc) Add(a, b int) int`; the old
  `(params *P, result *R) error` signature is no longer registered.
- **Breaking**: params travel as a positional JSON array
  (`"params":[1,6]`) instead of a named object.
- **Breaking**: `client.Client.Call` is now `Call(method string, args ...any)
  (any, error)`; `BatchAppend(method string, args ...any)` returns nothing and
  `BatchCall()` returns `([]BatchResult, error)`.
- **Breaking**: `common.Request.Id`, `common.SuccessResponse.Id` and
  `common.ErrorResponse.Id` are `any` instead of `string` so numeric
  JSON-RPC ids (sent by the MCP SDK) round-trip with their original type
  instead of panicking.

### Added
- Added `client.Client.Notify(method string, args ...any) error` for
  one-way notifications.
- Added the generic helper `jsonrpc4go.Call[T](c, method, args...) (T, error)`
  that decodes the result directly into `T`.
- Added the `streamable-http` protocol: a single `/mcp` endpoint implementing
  the MCP Streamable HTTP transport (JSON unary results, SSE streaming
  results, `Mcp-Session-Id` sessions, `Mcp-Protocol-Version` negotiation,
  `Last-Event-ID` replay, GET SSE channel, `202` notifications, `Origin`
  allow-list) plus the MCP layer (`initialize`, `ping`, `tools/list`,
  `tools/call` with `_meta.progressToken`), exposing registered methods as
  `Service_Method` tools while keeping legacy `Service/method` dispatch.
- Added streaming semantics: methods returning slices, arrays, channels or
  `iter.Seq`/`iter.Seq2` iterators are pushed value by value on streaming
  transports and collected into arrays on tcp/http.
- Added `client.Streamer` with `Stream(method string, args ...any)
  iter.Seq2[any, error]` and the generic helper
  `jsonrpc4go.Stream[T](c, method, args...) iter.Seq2[T, error]`.
- Added `server.StreamableHttpOptions.AllowedHosts`, an optional `Host`
  header allow-list that rejects other hosts with `421` to prevent DNS
  rebinding attacks against localhost servers.
- Added full MCP conformance support for protocol revision `2025-11-25`:
  server-to-client requests (`sampling/createMessage`, `elicitation/create`),
  `resources/list|read|templates/list|subscribe|unsubscribe`,
  `prompts/list|get`, `completion/complete`, `logging/setLevel`, log and
  progress notifications, SSE priming events with `retry` and
  Last-Event-ID resumption, and JSON Schema 2020-12 `inputSchema`
  preservation.
- Added `server.McpTool` with `RegisterMcpTool` for tools with an exact name,
  hand-written `inputSchema` and a handler receiving a `server.McpContext`
  (`Log`, `Progress`, `Request`, `EndStream`, `SendResult`), plus
  `RegisterMcpResource`, `RegisterMcpResourceTemplate` and
  `RegisterMcpPrompt`.
- Added `examples/streamable-http` server and client programs.
- Added an MCP conformance CI job using
  `modelcontextprotocol/conformance` with a documented
  `conformance-baseline.yml` of known failures.

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