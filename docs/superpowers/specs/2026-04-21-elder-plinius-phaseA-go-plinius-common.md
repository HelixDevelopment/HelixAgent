# Phase-A Plan — go-plinius-common (2026-04-21)

**Status:** GATED. Awaiting explicit approval.
**Parent design:** `docs/superpowers/specs/2026-04-21-remaining-work-execution-design.md` §Phase-4
**Index:** `docs/superpowers/specs/2026-04-21-elder-plinius-phaseA.md`
**Upstream Python:** To be located during brainstorm. The v3 codegen names this as a shared-utility package without a single canonical Python upstream — it aggregates plumbing that elder-plinius Python projects duplicate (config, errors, health checks, pagination, token accounting).
**Defensible-subset justification:** Per `docs/research/go-elder-plinius-v3_triage.md` §2 table: "Shared gRPC/config/errors helpers — Real impl from scratch (stubs don't compile)". Low-risk utility foundation that the other 8 defensible modules depend on; no dual-use surface.

## 1. Upstream behavioral surface

**Placeholder — to be derived during `superpowers:brainstorming` against
Python upstream.** Do NOT copy signatures from the v3 Go codegen
scaffold — semantic bugs contaminate its type signatures.

## 2. Proposed Go API (draft, from scaffold — unverified)

Based on `docs/research/go-elder-plinius-v3/go-elder-plinius/go-plinius-common/pkg/client/client.go`
and `pkg/types/types.go`, the scaffold exposes these (unverified) symbols:

- `pkg/config/config.go:20: type Config struct`
- `pkg/config/config.go:44: type Option func(*Config)`
- `pkg/config/config.go:47: func New(serviceName string, opts ...Option) *Config`
- `pkg/config/config.go:71: func WithAddress(addr string) Option`
- `pkg/config/config.go:76: func WithTimeout(d time.Duration) Option`
- `pkg/config/config.go:81: func WithConnectionTimeout(d time.Duration) Option`
- `pkg/config/config.go:86: func WithMaxRetries(n int) Option`
- `pkg/config/config.go:91: func WithRetryBackoff(d time.Duration) Option`
- `pkg/config/config.go:96: func WithMaxRetryBackoff(d time.Duration) Option`
- `pkg/config/config.go:101: func WithTLS(certPath, keyPath, caPath string) Option`
- `pkg/config/config.go:111: func WithInsecureSkipVerify(skip bool) Option`
- `pkg/config/config.go:116: func WithAuthToken(token string) Option`
- `pkg/config/config.go:121: func WithCompression(algo string) Option`
- `pkg/config/config.go:126: func WithKeepalive(time_, timeout time.Duration) Option`
- `pkg/config/config.go:134: func WithMaxMessageSize(bytes int) Option`
- `pkg/config/config.go:142: func WithMetadata(key, value string) Option`
- `pkg/config/config.go:153: func FromEnv(serviceName string) *Config`
- `pkg/config/config.go:234: func FromFile(path string, serviceName string) (*Config, error)`
- `pkg/config/config.go:269: func (c *Config) Validate() error`
- `pkg/config/config.go:295: func (c *Config) EnvPrefix() string`
- `pkg/errors/errors.go:15: type ErrorCode string`
- `pkg/errors/errors.go:70: type PliniusError struct`
- `pkg/errors/errors.go:94: func (e *PliniusError) Error() string`
- `pkg/errors/errors.go:102: func (e *PliniusError) Unwrap() error`
- `pkg/errors/errors.go:107: func (e *PliniusError) WithCause(cause error) *PliniusError`
- `pkg/errors/errors.go:113: func (e *PliniusError) WithDetail(key string, value interface{}) *PliniusError`
- `pkg/errors/errors.go:122: func (e *PliniusError) IsRetryable() bool`
- `pkg/errors/errors.go:127: func New(code ErrorCode, service, message string) *PliniusError`
- `pkg/errors/errors.go:137: func Wrap(code ErrorCode, service, message string, cause error) *PliniusError`
- `pkg/errors/errors.go:142: func Newf(code ErrorCode, service, format string, args ...interface{}) *PliniusError`
- `pkg/errors/errors.go:148: func Is(err error, code ErrorCode) bool`
- `pkg/errors/errors.go:157: func IsRetryableError(err error) bool`
- `pkg/errors/errors.go:170: var (` (sentinel error codes block)
- `pkg/errors/errors.go:177: func isDefaultRetryable(code ErrorCode) bool`
- `pkg/errors/errors.go:195: func MustBePliniusError(err error) *PliniusError`
- `pkg/grpcclient/grpcclient.go:30: type Client struct`
- `pkg/grpcclient/grpcclient.go:38: func New(cfg *config.Config) *Client`
- `pkg/grpcclient/grpcclient.go:44: func (c *Client) Connect(ctx context.Context) error`
- `pkg/grpcclient/grpcclient.go:79: func (c *Client) Connection() *grpc.ClientConn`
- `pkg/grpcclient/grpcclient.go:86: func (c *Client) Close() error`
- `pkg/grpcclient/grpcclient.go:104: func (c *Client) IsConnected() bool`
- `pkg/grpcclient/grpcclient.go:116: func (c *Client) WaitForReady(ctx context.Context) error`
- `pkg/grpcclient/grpcclient.go:141: func (c *Client) InvokeUnary(...)`
- `pkg/grpcclient/grpcclient.go:187: func (c *Client) ContextWithMetadata(ctx context.Context) context.Context`
- `pkg/grpcclient/grpcclient.go:207: func (c *Client) buildDialOptions() []grpc.DialOption`
- `pkg/grpcclient/grpcclient.go:259: func (c *Client) retryInterceptor() grpc.UnaryClientInterceptor`
- `pkg/grpcclient/grpcclient.go:268: func calculateBackoff(base, max time.Duration, attempt int) time.Duration`
- `pkg/types/types.go:13: type ServiceClient interface`
- `pkg/types/types.go:31: type HealthStatus struct`
- `pkg/types/types.go:52: type HealthCheck struct`
- `pkg/types/types.go:67: func (h *HealthStatus) IsHealthy() bool`
- `pkg/types/types.go:72: type Pagination struct`
- `pkg/types/types.go:84: type PaginatedResponse struct`
- `pkg/types/types.go:102: type VersionInfo struct`
- `pkg/types/types.go:123: func (v *VersionInfo) String() string`
- `pkg/types/types.go:128: type TokenUsage struct`
- `pkg/types/types.go:135: type ScoreBreakdown struct`
- `pkg/types/types.go:146: type ProgressUpdate struct`
- `pkg/types/types.go:167: type ProgressCallback func(update ProgressUpdate)`
- `pkg/types/types.go:170: type StageResult struct`
- `pkg/types/types.go:188: type PipelineResult struct`
- `pkg/types/types.go:200: type StreamOption struct`

These are starting points for review, NOT implementation commitments.

## 3. Core-surface scope (4 days)

Proposed subset of §2 to implement for Phase-A core:

- `ErrorCode` sentinels + `PliniusError` wrapper (`Error`, `Unwrap`, `WithCause`, `WithDetail`, `IsRetryable`) plus constructors (`New`, `Wrap`, `Newf`, `Is`).
- `HealthStatus` + `HealthCheck` + `IsHealthy` for uniform health reporting across the other 8 modules.
- `TokenUsage` + `ScoreBreakdown` shared accounting types (consumed by autotemp, hypertune, v3r1t4s).

## 4. Full-spec scope (2 weeks)

Everything beyond core:

- `Config` builder with `Option`s (including TLS, keepalive, metadata, auth, compression), `Validate`, `EnvPrefix`, `FromEnv`, `FromFile` loaders.
- `Pagination` + `PaginatedResponse` generic wrappers.
- `VersionInfo` + `String` formatter.
- `ProgressUpdate` + `ProgressCallback` + `StageResult` + `PipelineResult` staged-run reporting (used by hypertune/autotemp long runs).
- `StreamOption` streaming primitives.
- gRPC client scaffolding (`grpcclient`) — **optional**; drop if internal consumers use HelixAgent's existing HTTP/3 stack instead of introducing a second transport. Decision deferred to brainstorm.

## 5. Test plan

- **Unit:** per-function table tests (target: 100% coverage per CLAUDE.md §1).
- **Integration:** interaction with all other 8 defensible modules (they import this package; integration tests verify they can construct/propagate errors and share token/score types end-to-end).
- **Fixture contributions:** N/A — utility module, no attack-pattern output.

## 6. Integration point

Intended consumer: all other 8 defensible modules.
Likely path: `internal/elder_plinius/common/` OR a top-level submodule.
Decided during brainstorm.

## 7. Documentation deliverables

- `CLAUDE.md` (per CLAUDE.md §7).
- `AGENTS.md`.
- `README.md`.
- `docs/` module documentation.

## 8. Risks

- Introducing gRPC transport duplicates HelixAgent's existing HTTP/3+Brotli stack (Constitution §Networking); gRPC scaffolding may need to be dropped in favor of a thin shim over the existing transport.
- Premature API lock-in: because the other 8 modules depend on this, any signature churn in the common types forces simultaneous refactors across all of them.

## 9. Approval checkpoint

> "Approve Phase-A for go-plinius-common — INTERNAL only, no public repo,
>  clean-room re-implementation from Python upstream."
