# GoSite release regression matrix

**Status:** Living checklist — update when adding panel views or changing critical flows.

**Executor:** [gosite-release-qa](../../.agents/skills/gosite-release-qa/SKILL.md) skill (Playwright MCP, admin persona).

**Not this doc:** Layperson UX confusion — use `qa-layperson-ui-ux` after functional gate passes.

## Severity

| Level | Blocks tag? | Examples |
|-------|-------------|----------|
| **Blocker** | Yes — no `vX.Y.Z-rc.1` or `vX.Y.Z` | Login fail, dashboard error boundary, `gosite/mcp` missing from registry, Enable fails |
| **Major** | Blocks final `vX.Y.Z`; OK for `-rc.N` with documented waiver | Website CRUD broken, nginx reload feedback wrong, file browser unusable |
| **Minor** | No — release notes | Copy, spacing, non-critical empty states |
| **UX** | No — defer to layperson QA | Confusing labels without functional failure |

## Modes

| Mode | TC scope | When |
|------|----------|------|
| `smoke` | Rows marked **smoke** (~8) | Large PR, pre-merge sanity |
| `full` | All rows | Before `vX.Y.Z-rc.1` and before final `vX.Y.Z` |
| `delta` | Changed areas + all **smoke** rows | PR with known touch map |

## Preconditions (setup — not test shortcuts)

1. Target running: `make dev-api` (local) or post-deploy panel URL (VM).
2. Version recorded: `X.Y.Z` or `X.Y.Z-dev` from Settings / About or build metadata.
3. Bundled gate: `gosite/mcp` appears in Plugins registry as **installed** (disabled by default). Deploy script verifies CLI; full QA verifies UI.
4. Credentials from README/env — login **via UI only** during the session.

Default admin (first init): `admin@demo.com` / `123456`.

Cloud lab (`scripts/compose.cloud-lab.yml`): `DEMO_SEED=false` — fresh install has **no** demo websites; create test domains via panel only.

## Matrix

Navigate via **sidebar** only (hash router). Do not deep-link except base app URL.

### Login / session

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-AUTH-01 | Blocker | smoke | Open panel → login form → valid credentials → Sign in | Lands on Dashboard; no persistent error toast |
| TC-AUTH-02 | Major | full | Sign out (if available) → login again | Session restored; sidebar visible |
| TC-AUTH-03 | Minor | full | Invalid password once | Actionable error message; form still usable |

### Dashboard

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-DSH-01 | Blocker | smoke | Sidebar → Dashboard | Page loads without error boundary; primary widgets/cards render |
| TC-DSH-02 | Minor | full | Scan layout on Dashboard | No overlapping/collapsed critical controls |

### Websites

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-WEB-01 | Blocker | smoke | Sidebar → Websites | List or empty state loads; no error boundary |
| TC-WEB-02 | Major | full | Open add/create website flow (if shown) | Form opens; required fields labeled |
| TC-WEB-03 | Major | full | Create test site (name + path under web root) or cancel cleanly | Save succeeds with feedback, or cancel returns to list |
| TC-WEB-04 | Major | delta | Toggle site enabled/disabled (if row action exists) | State changes with visible feedback |
| TC-WEB-05 | Minor | full | Delete/cleanup test site (if created) | Confirm dialog; row removed or archived |

### Files

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-FIL-01 | Blocker | smoke | Sidebar → Files | Browser loads root or allowed path; not hard error on first paint |
| TC-FIL-02 | Major | full | Navigate into a subdirectory via UI | Path/breadcrumb updates; listing refreshes |
| TC-FIL-03 | Major | full | Upload or create file (if UI offers) | Progress/result feedback |
| TC-FIL-04 | Minor | full | Return to parent via breadcrumb | Correct directory shown |

### Nginx

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-NGX-01 | Blocker | smoke | Sidebar → Nginx | Config/status view loads |
| TC-NGX-02 | Major | full | Reload or apply action (if exposed) | Success or explicit error toast — not silent fail |
| TC-NGX-03 | Minor | full | Validate/test action (if exposed) | Result message visible |

### Docker

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-DKR-01 | Blocker | smoke | Sidebar → Docker | Container list or empty state; no error boundary |
| TC-DKR-02 | Major | full | Inspect one container row / detail | Name, status, or actions visible |
| TC-DKR-03 | Minor | full | Start/stop/restart (non-production test container only) | Feedback matches action |

### Mounts

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-MNT-01 | Major | full | Sidebar → Mounts | Table or empty state loads |
| TC-MNT-02 | Major | full | Add mount wizard (dry run / cancel if no safe test path) | Form validation messages understandable |
| TC-MNT-03 | Minor | full | Cancel out of add flow | Returns to list unchanged |

### Cron

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-CRN-01 | Major | full | Sidebar → Cron | Job list or empty state loads |
| TC-CRN-02 | Minor | full | Open add job form | Fields visible; cancel works |

### Database

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-DB-01 | Major | full | Sidebar → Database | SQLite browser or status loads |
| TC-DB-02 | Minor | full | Run read-only query or browse tables (if offered) | Results or empty state |

### Logs

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-LOG-01 | Major | full | Sidebar → Logs | Query UI loads; default tail/search works |
| TC-LOG-02 | Minor | full | Change time range or filter | Results update or explicit empty |

### Metrics / Traffic

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-MET-01 | Major | full | Sidebar → Traffic (Metrics) | Charts or metrics placeholders load |
| TC-MET-02 | Minor | full | Change interval/range if available | Chart refreshes |

### Settings

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-SET-01 | Major | full | Sidebar → Settings | Account/settings form loads |
| TC-SET-02 | Minor | full | Version/build string visible | Matches expected `X.Y.Z` or `X.Y.Z-dev` |
| TC-SET-03 | Minor | full | Save harmless setting or cancel | No silent failure |

### Plugins (registry)

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-PLG-01 | Blocker | smoke | Sidebar → Plugins → registry list | `gosite/mcp` row present, status **installed** |
| TC-PLG-02 | Blocker | full | Open `gosite/mcp` detail | Version, hooks/capabilities summary visible |
| TC-PLG-03 | Blocker | smoke | Inspect `gosite/mcp` row badges | **built-in** badge shown; source indicates bundled |
| TC-PLG-04 | Blocker | smoke | Enable `gosite/mcp` from registry | Status becomes enabled; no blocker error toast |
| TC-PLG-05 | Major | full | Disable `gosite/mcp` then re-enable | State toggles with feedback |
| TC-PLG-06 | Major | full | MCP integration indicator (if panel shows tools/status after enable) | MCP reachable or status explained in UI |

### Plugins (catalog / install)

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-PLG-INST-01 | Major | full | Open catalog → find `gosite/mcp` | **built-in** badge; copy says seed on init, not remote install |
| TC-PLG-INST-02 | Major | delta | Install flow for non-bundled catalog entry (optional) | Resolve/download works or clear error |
| TC-PLG-INST-03 | Minor | full | Close catalog without install | Returns to registry |

### Navigation / shell

| TC-ID | Severity | Mode | Steps | Expected |
|-------|----------|------|-------|----------|
| TC-NAV-01 | Major | full | Visit each sidebar group once | Every route in sidebar renders without crash |
| TC-NAV-02 | Minor | full | Brand/logo → Dashboard | Returns to home view |

## Coverage rules

| Mode | Required coverage |
|------|-------------------|
| `smoke` | 100% of **smoke** rows |
| `full` | 100% of all rows (waive **Minor** only with explicit note in SUMMARY) |
| `delta` | 100% **smoke** + 100% rows in touched areas |

## Output artifacts

Write under `logs/qa-release-{version}-{mode}/` (gitignored):

- `00-SUMMARY.md` — gate decision, coverage %, blockers
- `matrix-results.md` — one row per executed TC-ID
- `screenshots/` — failures and blocker passes

See [examples.md](../../.agents/skills/gosite-release-qa/examples.md).

## Related

- Versioning gate: [gosite-versioning](../../.agents/skills/gosite-versioning/SKILL.md)
- Bundled plugins: `docs/sequences/23-builtin-plugins.md`
- MCP operator: `docs/guides/mcp-operator.md`
- Deploy verify: `scripts/deploy-vm.example.sh`
- Layperson UX (after functional pass): `qa-layperson-ui-ux` skill
