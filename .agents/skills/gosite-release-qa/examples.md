# Release QA — example artifacts

## 00-SUMMARY.md

```markdown
# Release QA Summary

| Field | Value |
|-------|-------|
| Gate | **PASS** |
| Version | 1.0.0-dev |
| Mode | smoke |
| Target | http://127.0.0.1:8080 |
| Date | 2026-06-18 |
| Executor | agent (gosite-release-qa) |

## Coverage

| Metric | Value |
|--------|-------|
| Required TCs | 8 |
| Executed | 8 |
| Passed | 8 |
| Failed | 0 |
| Skipped | 0 |
| Coverage | 100% |

## Blockers

None.

## Major / Minor notes

- TC-NGX-02 skipped: reload button not exposed in dev-api nginx stub.

## Recommendation

Safe to proceed toward `v1.0.0-rc.1` after full matrix on rc image.
```

## matrix-results.md

```markdown
# Matrix results — 1.0.0-dev smoke

| TC-ID | Severity | Result | Notes | Screenshot |
|-------|----------|--------|-------|------------|
| TC-AUTH-01 | Blocker | Pass | Login → Dashboard | screenshots/TC-AUTH-01-pass.png |
| TC-DSH-01 | Blocker | Pass | Widgets visible | screenshots/TC-DSH-01-pass.png |
| TC-WEB-01 | Blocker | Pass | Empty list OK | — |
| TC-FIL-01 | Blocker | Pass | Root listing | — |
| TC-NGX-01 | Blocker | Pass | Config view | — |
| TC-DKR-01 | Blocker | Pass | Docker list | — |
| TC-PLG-01 | Blocker | Pass | gosite/mcp installed | screenshots/TC-PLG-01-pass.png |
| TC-PLG-03 | Blocker | Pass | built-in badge | screenshots/TC-PLG-03-pass.png |
| TC-PLG-04 | Blocker | Pass | Enable succeeded | screenshots/TC-PLG-04-pass.png |
```

## FAIL example (blocker)

```markdown
# Release QA Summary

| Field | Value |
|-------|-------|
| Gate | **FAIL** |
| Version | 1.0.0 |
| Mode | smoke |

## Blockers

1. **TC-PLG-01** — `gosite/mcp` not in Plugins registry after deploy.
2. **TC-PLG-04** — Enable not attempted (blocked by PLG-01).

## Recommendation

Do not tag. Check bundled seed (`PLUGIN_BUNDLED_PATH`, bootstrap.log) and redeploy with `scripts/deploy-vm.example.sh` verify step.
```
