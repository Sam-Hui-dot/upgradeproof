# Real OSS validation interim decision

Decision after the first three repositories: **PIVOT, then continue evidence collection**.

This is not a KILL signal. Two materially different common topologies completed real pinned upgrades without core changes:

- Savvy: one application container with SQLite in a named volume, `v1.2.2 -> v1.2.3`.
- SleepLab: one versioned application container plus stable PostgreSQL, `v1.3.1 -> v1.4.0` with target migrations.

Spliit Cloud exposed a real boundary: one release is a coordinated set of migrate/API/worker/web images. The current one-versioned-service operation upgraded only API, leaving the other components and target migration behind. The red run correctly detected this rather than being made green with hook-driven orchestration.

Therefore:

1. Preserve the validated single-service core abstraction for now.
2. Record coordinated release/service groups as the leading evidence-backed pivot candidate; do not implement it during this validation pass.
3. Continue Labby and Notifuse because their requested value is orthogonal: they test whether the current safety contract is a reasonable boundary or a practical adoption blocker.
4. Do not claim general compatibility or external adoption.

Snapshot at this decision:

| Repository | Result without core change | Signal |
| --- | --- | --- |
| Savvy | passed | single-container persistent state is expressible |
| SleepLab | passed | app + stable database is expressible |
| Spliit Cloud | failed as expected | coordinated multi-image release is not expressible |

The failure is important but not yet systemic across the validated target class. The next two safety-focused validations can still change the final GO/PIVOT/KILL recommendation.
