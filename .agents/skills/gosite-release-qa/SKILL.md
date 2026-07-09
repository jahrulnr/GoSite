---
name: gosite-release-qa
description: Release regression QA for GoSite admin panel before version tags. Matrix-driven UI tests via Playwright MCP (user-playwright); smoke/full/delta modes; blocker gate. Use before vX.Y.Z-rc, after deploy, or when user asks release QA / regression test. Not layperson UX — see qa-layperson-ui-ux.
---

# GoSite release QA

Admin-operator regression against [docs/qa/release-matrix.md](../../../docs/qa/release-matrix.md). **Functional gate** before tagging — not awam UX (that is `qa-layperson-ui-ux`).

## Persona

**Admin operator** — understands panel terms (nginx, plugins, Docker). Executes **UI only** via Playwright MCP (`user-playwright`). May read README/env **once** before the session for URL and credentials; during the session: no `curl`, API calls, or source reads.

## Modes

| Mode | Scope | When | Budget |
|------|-------|------|--------|
| `smoke` | Matrix rows marked **smoke** | Large PR, quick pre-merge | ~20 min |
| `full` | Entire matrix | Before `vX.Y.Z-rc.1` and final `vX.Y.Z` | ~60–90 min |
| `delta` | Changed areas + all **smoke** | PR with known touch map | ~30 min |

## Release gate

| Level | Blocks tag? |
|-------|-------------|
| **Blocker** | Yes — stop; no rc/final tag |
| **Major** | Blocks final `vX.Y.Z`; document waiver for `-rc.N` |
| **Minor** | No — release notes |
| **UX** | Defer to layperson skill |

Blockers include: login fail, dashboard error boundary, `gosite/mcp` missing from registry, Enable fails, any **smoke** row fail.

## Workflow

### 0. Read matrix

Open `docs/qa/release-matrix.md`. Build the TC list for the chosen mode:

- `smoke` → filter `Mode` column contains `smoke`
- `full` → all TC-IDs
- `delta` → smoke rows + rows in areas matching PR/touch map

### 1. Preconditions (setup only)

```text
1. Target: make dev-api OR post-deploy panel URL (IP: https://<vm-ip>:1100/)
2. Record version: X.Y.Z or X.Y.Z-dev (Settings / About)
3. Confirm bundled seed: gosite/mcp installed (UI TC-PLG-01; CLI only in deploy setup, not during QA session)
4. Credentials: default admin `admin@demo.com` / `123456` (seeded on first init). Cloud lab uses `DEMO_SEED=false` — no demo websites/logs; create `example.bangunsoft.com` via Websites UI before site checks.
```

### 2. Open Playwright MCP (single session)

Use `user-playwright` tools on **one tab** for the entire run.

| Do | Don't |
|----|-------|
| `browser_navigate` once to panel base URL | `browser_run_code_unsafe` (opens extra browser windows) |
| Reuse the same page for all TCs | `newContext()` / `newPage()` / per-TC scripts |
| `browser_snapshot` → click/type on returned refs | Chain multiple isolated Playwright scripts that each restart the browser |

**Panel URL:** `http://<vm-ip>:1100/` (cloud lab: `TLS_ENABLE=false` — no self-signed cert). Do not use domain for panel login.

**Websites / domains:** create test sites (e.g. `example.bangunsoft.com`) via **Websites → New** in the panel — not via CLI, API, or manual nginx on the VM.

Navigate via **sidebar** only (hash router). Do not deep-link except base app URL.

### 3. Execute matrix

For each TC-ID in order:

1. State TC-ID and severity aloud in notes
2. Perform steps from matrix **via UI**
3. Record Pass / Fail / Skip (+ reason)
4. Screenshot on Fail; screenshot Blocker on Pass
5. On Blocker fail → stop session; write SUMMARY with gate **FAIL**

**Forbidden during session:** curl, shell API, reading source, deep URLs (`#/files?path=...`), cookie injection, skipping login.

### 4. Write artifacts

Directory: `logs/qa-release-{version}-{mode}/`

| File | Content |
|------|---------|
| `00-SUMMARY.md` | Gate, version, target URL, mode, coverage %, blocker list |
| `matrix-results.md` | Table: TC-ID, severity, result, notes, screenshot path |
| `screenshots/` | `TC-*-{pass\|fail}.png` |

Templates: [examples.md](examples.md).

### 5. Gate decision

```text
PASS → no Blocker fails; smoke/full coverage rules met (see matrix)
FAIL → any Blocker fail OR full mode <100% required rows
WAIVE → Major only, documented for -rc.N (not for final vX.Y.Z)
```

Link SUMMARY in release notes or PR when gating a tag.

## Integration with release pipeline

```text
PR candidate → smoke (AI or future CI)
  → tag vX.Y.Z-rc.1 + deploy
  → full on rc image
  → fix blockers
  → tag vX.Y.Z
  → optional qa-layperson-ui-ux
```

Couple with [gosite-versioning](../gosite-versioning/SKILL.md) release checklist.

## Playwright MCP tips

- Prefer `getByRole`, visible text, sidebar labels over brittle CSS
- Wait for network idle or visible heading after navigation
- If element not found: one retry with scroll; then Fail with screenshot
- Long-term: add `data-testid` on critical plugin/auth controls

## Phase 2 (not required for skill)

- `web/e2e/smoke/` Playwright tests mirroring smoke TCs
- CI job on PR for deterministic smoke

## Related

| Asset | Role |
|-------|------|
| [release-matrix.md](../../../docs/qa/release-matrix.md) | Source of truth for TCs |
| [gosite-versioning](../gosite-versioning/SKILL.md) | SemVer, deploy, `-dev` |
| `qa-layperson-ui-ux` | Post-functional UX pass |
| `scripts/deploy-vm.example.sh` | Post-deploy CLI bundled verify |
| `docs/guides/mcp-operator.md` | MCP enable expectations |
