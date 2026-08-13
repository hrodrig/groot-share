# Phase 6: Housekeeping - Context

**Gathered:** 2026-08-12
**Status:** Ready for planning

<domain>
## Phase Boundary

Audit rows for upload/download/delete (no secrets). Retention job deletes home objects when **either** keep_last **or** max_age_days fires (defaults 20 / 90). On `vps-s3`, retention deletes bucket objects. Staging leftovers are incidents, not the retention set.

</domain>

<spec_lock>
## Requirements (locked via SPEC)

AUD-01, RET-01.

**In scope:** sqlite `audit`; GET `/v1/audit`; DELETE archive; in-process sweep; `GFS_KEEP_LAST` / `GFS_MAX_AGE_DAYS`; blob.Delete.

**Out of scope:** SPA polish, per-user retention overrides, trusted-proxy IP, analyze/LLM, give-up policy for transit.

</spec_lock>

<decisions>
## Implementation Decisions

- **D-01:** Audit columns: actor (username), action (`upload`|`download`|`delete`), object id/key, ts, remote IP (`RemoteAddr`). Never password or api_key. Retention deletes use actor `retention`.
- **D-02:** `GET /v1/audit` JSON (session). Home HTML shows recent rows. Authenticated users see all (visibility later).
- **D-03:** `DELETE /v1/archives/{id...}` and HTML `POST .../delete`. VPS: file + sqlite row. `vps-s3`: `DeleteObject` on the key (home).
- **D-04:** Delete when index ≥ keep_last (newest-first) **or** created_at older than max_age_days (union). Defaults 20 / 90.
- **D-05:** In-process ticker (default 1h); tests call `SweepOnce`. Staging older than grace (default 24h) is logged and removed, not counted in keep_last.

</decisions>

<canonical_refs>
- `docs/SPECIFICATIONS.md` §7 retention, §8 audit
- `docs/GFS-CONSENSUS.md` decisions 1 and 8
- `.planning/REQUIREMENTS.md` — AUD-01, RET-01
</canonical_refs>

<deferred>
- `/gsd-ui-phase` visual contract
- Transit give-up / operator alert
</deferred>

---

*Phase: 6-Housekeeping*
*Context gathered: 2026-08-12*
