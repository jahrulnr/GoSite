# Tier 3 — native `.so` (vendor-only)

**Not for community plugins.** GoSite deliberately prefers Tier 1 subprocess
isolation over Go's `plugin` package (`.so` ABI coupling to the exact host
Go version and build flags).

## When Tier 3 might be used

- In-tree vendor extensions shipped **with** the same GoSite release binary
- Hardware or libc bindings that cannot run in a subprocess sandbox
- Temporary bridge during migration from legacy `.so` extensions

## Risks

- ABI break on every GoSite upgrade (rebuild required)
- Shared memory with host — memory safety bugs become host RCE
- No cross-platform artifacts (linux/amd64 vs arm64)

## Recommendation

Copy [`tier1-full`](../tier1-full/) or [`tier1-minimal`](../tier1-minimal/)
instead. Use Tier 0 webhooks when logic must live outside the host entirely.

There is no `manifest.json` template here — the host does not accept tier 3
installs in the current implementation wave.
