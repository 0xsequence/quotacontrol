# QuotaControl

Quota management and rate limiting library for 0xsequence services.

## Quick Reference

```bash
make test                    # Run all tests (race, shuffle, failfast)
make test TEST=TestName      # Run specific test
make lint                    # Lint + auto-fix (golangci-lint v2)
make generate                # Regenerate proto (webrpc from .ridl)
```

## Architecture

**Two-actor model:**
- **Server** (`server.go`): Persists quota records via storage interfaces (`ProjectInfoStore`, `LimitStore`, `AccessKeyStore`, `UsageStore`, `PermissionStore`). Implements `proto.QuotaControlServer`.
- **Client** (`client.go`): Reads from cache, syncs usage in background. Provides HTTP middleware for chi routers.

**Middleware chain** (in `middleware/`): SetCost → VerifyQuota → RateLimit → EnsureUsage → SpendUsage

**Cache** (`cache/`): Redis primary + optional LRU overlay. Cache keys are versioned (except usage counters). Usage spending uses atomic Redis Lua scripts.

**Proto** (`proto/`): Source of truth is `proto/quotacontrol.ridl` (WebRPC schema). Generated files: `*.gen.go`, `*.gen.ts`. Never edit generated files directly.

## Code Conventions

- **Go version**: 1.23
- **Import order** (enforced by `gci`): standard → third-party → `github.com/0xsequence/quotacontrol`
- **Error naming**: sentinel errors prefixed with `Err`, error types suffixed with `Error`
- **Logging**: `log/slog` structured logging
- **Context**: custom context keys for quota, cost, time, spending (see `middleware/context.go`)
- **Pointers for optionals**: use `proto.Ptr()` helper
- **Testing**: `testify` (assert/require) + `miniredis` for Redis tests
- **WebRPC**: type renames are wire-safe — only field and argument names matter in client/server comms

## Key Packages

| Package | Purpose |
|---------|---------|
| root | Client, Server, Config types |
| `proto/` | WebRPC schema, generated types, validation helpers |
| `middleware/` | HTTP middleware (quota, rate limit, usage, permission, cost) |
| `cache/` | Redis and LRU cache backends |
| `mock/` | In-memory implementations for testing |
| `internal/` | Cycle calculations, usage sync internals |

## Key Types

- **`Limit`** (formerly `ServiceLimit`): Per-service limit config (rateLimit, freeWarn, freeMax, overWarn, overMax)
- **`LegacyLimit`** (formerly `Limit`): Map of service→Limit, deprecated — being replaced by per-service queries
- **`ProjectInfo`**: Project-level config (projectID, ecosystemID, chainIDs, cycle, services) — replaces `CycleStore`
- **`AccessQuota`**: Composite of cycle + legacyLimit + accessKey — legacy type being decomposed into individual endpoints

## Store Interfaces

| Interface | Purpose |
|-----------|---------|
| `ProjectInfoStore` | Project-level config (replaces CycleStore) |
| `LimitStore` | Per-service limits: `GetLimit(ctx, projectID, service)` |
| `AccessKeyStore` | Access key CRUD and lookup |
| `UsageStore` | Usage records and sync |
| `PermissionStore` | User permissions per project |

## Cache Internals

**Layered architecture:** LRU (optional) → Redis → server API. LRU enabled when `Config.LRUSize > 0`.

**Generic interfaces** (`cache/common.go`):
- `Simple[K Key, T any]` — Get/Set/Clear. `Get` returns `(value, ok, err)` where `ok=false` means miss (not error).
- `Usage[K Key]` — Extends Simple with `Ensure(ctx, fetcher, key)` and `Spend(ctx, fetcher, key, amount, limit)`.
- `Fetcher[K]` = `func(ctx, key) (int64, error)` — lazy-loads initial counter value from server.

**Key versioning** (`keys.go`): All keys except `KeyUsage` include `Version = "v2"` in their string representation. Bump `Version` to bust all caches. Usage counters are unversioned (raw counters, safe across versions).

**Key format examples:**
- `quota:v2:{AccessKey}` — legacy access quota
- `limit:v2:{ProjectID}:{Service}` — per-service limit (v2)
- `usage:{Service}:{ProjectID}:{YYYY-MM-DD}--{YYYY-MM-DD}` — usage counter (no version)

**Atomic spend:** `Spend` uses a Redis Lua script that atomically increments the counter, caps at limit, and returns `[newValue, delta]`. Counter never exceeds limit under concurrency.

**Ensure flow:** Uses sentinel value `-1` as initialization lock. Retries with backoff (100ms, 200ms, 300ms) if another client is initializing.

## Client Lifecycle

**State machine** (atomic `running` field): `0` (stopped) → `1` (running) → `2` (stopping) → `0`.

**Run(ctx):** Blocking loop on ticker (default 5min). Each tick calls `usage.SyncUsage()` to flush accumulated counters to server via `SyncAccessKeyUsage`/`SyncProjectUsage` RPCs. Failed syncs are re-queued.

**Stop(ctx):** Stops ticker, runs one final `SyncUsage()` to flush pending data, then returns. Must be called for clean shutdown — otherwise pending usage is lost.

**In tests:** Always `go client.Run(ctx)` in a goroutine, then `defer client.Stop(ctx)`. Call `Stop` before asserting on server-side usage to ensure sync completes.

## Middleware Data Flow

Context is the data bus between middleware steps. All reads/writes via exported functions in `middleware/context.go`.

| Step | Reads from context | Writes to context |
|------|-------------------|-------------------|
| **SetCost** | — | `AddCost(ctx, n)` (additive, also sets `httprate.WithIncrement`) |
| **VerifyQuota** | session type, access key, origin | `AccessQuota`, `ServiceLimit`, project ID |
| **RateLimit** | project ID, account, cost | — (checks httprate counters) |
| **EnsureUsage** | `AccessQuota`, `ServiceLimit`, cost | — (denies if `usage + cost > limit.OverMax`) |
| **SpendUsage** | `AccessQuota`, `ServiceLimit`, cost | `withSpending` flag, response headers (`Quota-Remaining`, `Quota-Cost`) |

**VerifyQuota** selects fetch path by session type: `FetchProjectQuota` (project JWT) or `FetchKeyQuota` (access key header). Both validate chains and set `ServiceLimit` on context.

**SpendUsage** calls `Client.SpendUsage()` which returns `(spent bool, total int64, err)`. If `spent < cost`, returns `ErrQuotaExceeded`. On threshold crossings (FreeWarn/FreeMax/OverWarn/OverMax), fires `NotifyEvent` RPC.

## Config

```go
Config {
  Enabled       bool           // master switch
  URL           string         // quotacontrol server URL
  AuthToken     string         // Bearer token for server auth
  UpdateFreq    time.Duration  // sync interval (default 5m)
  DefaultUsage  *int64         // cost per request (default 1)
  LRUSize       int            // in-memory cache entries (0 = disabled)
  LRUExpiration time.Duration  // LRU entry TTL
  DangerMode    bool           // debug/testing flag
  Redis         RedisConfig    // host, port, dbIndex, maxIdle, keyTTL
  RateLimiter   RateLimitConfig // enabled, publicRPM, accountRPM, serviceRPM
}
```

## Testing Patterns

**Setup:** `mock.NewServer(&cfg)` returns `(*Server, cleanup)`. Spins up miniredis + in-memory store + HTTP listener on random port. Mutates `cfg.URL` and `cfg.Redis` in-place. Always `t.Cleanup(cleanup)`.

**Key helpers** (`server_test.go`):
- `newConfig()` — minimal working Config (enabled, 1m update freq, redis + ratelimiter on)
- `executeRequest(ctx, handler, path, accessKey, jwt)` → `(ok, headers, error)` — simulates HTTP POST with optional auth
- `hitCounter` / `spendingCounter` — atomic counters as `http.Handler` for tracking request flow

**Mock server features** (`mock/server.go`):
- `server.Store` — direct access to `MemoryStore` for test setup (SetLimit, InsertAccessKey, etc.)
- `server.FlushCache(ctx)` — Redis FLUSHALL to force cache misses
- `server.GetEvents(projectID)` — returns accumulated `[]Event{Service, Type}` from NotifyEvent calls
- `server.ErrGetProjectQuota` / `server.ErrGetAccessQuota` — set to inject errors on quota fetch

**Time control:** `middleware.WithTime(ctx, now)` overrides `time.Now()` for deterministic cycle calculations.

## Linting

Uses `golangci-lint` v2 with: govet, errcheck, errorlint, staticcheck, unused, bodyclose, errname, exptostd, fatcontext, usestdlibvars. Generated files (`*.gen.go`) are excluded.

## Consumed By

node-gateway, relayer, metadata, indexer, marketplace-api, trails-api, guard, stack — changes here have wide blast radius.
