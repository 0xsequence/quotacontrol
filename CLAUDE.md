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

## Linting

Uses `golangci-lint` v2 with: govet, errcheck, errorlint, staticcheck, unused, bodyclose, errname, exptostd, fatcontext, usestdlibvars. Generated files (`*.gen.go`) are excluded.

## Consumed By

node-gateway, relayer, metadata, indexer, marketplace-api, trails-api, guard, stack — changes here have wide blast radius.
