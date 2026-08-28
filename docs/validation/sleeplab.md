# SleepLab compatibility validation

## Outcome

**Compatibility validated:** yes, for the explicitly tested `v1.3.1 -> v1.4.0` Docker Hub image path with persistent PostgreSQL state.

**External adoption:** no. This is a local compatibility prototype only; upstream has not accepted or adopted UpgradeProof.

No UpgradeProof core code or SleepLab business code was changed. Only validation Compose/config/seed/verify glue was added in the local clone.

## Repository and release evidence

| Field | Result |
| --- | --- |
| Repository | `joshuamyers-dev/sleeplab` (`https://github.com/joshuamyers-dev/sleeplab`) |
| Stars | 21, GitHub API snapshot at 2026-08-28 22:28 +08:00 |
| Audited revision | `72853f96ac44cb572a135f273065e7cde00046ee` (`main`) |
| Historical release | `v1.3.1`, published 2026-06-05 |
| Target release | `v1.4.0`, published 2026-06-08 |
| Source image | `joshuaaaronmyers/sleeplab@sha256:38a5501f1a030e1e734641b89c41707a85f94507101d1a03df93657a349977cc` |
| Target image | `joshuaaaronmyers/sleeplab@sha256:ac081067ee1b5dea9b893656847171fe5745d2b93ed1156b490edd7e4a0882da` |
| Upstream upgrade guidance | Pull the new image and run `docker compose up -d`; migrations run automatically at API startup |
| Realistic supported path | Direct image replacement on the retained PostgreSQL volume; no formal compatibility matrix, but this is the documented deployment model |

`v1.3.1 -> v1.4.0` was selected over the latest adjacent patch pair because it exercises two real target migrations: `022_add_adherence_settings.sql` and `023_add_adherence_enabled.sql`.

## Architecture audit

- Compose topology: `app` plus PostgreSQL 16. The app image runs nginx and FastAPI/uvicorn under one entrypoint.
- Persistent resources: one project-scoped named PostgreSQL data volume. The app container is stateless in the minimal supported topology; the advanced topology optionally adds a read-only host bind for CPAP data.
- Migration mechanism: `server.py` creates a `schema_migrations` ledger, baselines pre-ledger schema where necessary, and applies `schema.sql` plus ordered SQL files transactionally at import/startup.
- Existing tests: backend pytest against a PostgreSQL CI service, frontend Vitest/build/lint, Ruff, and a migration-safety workflow that enforces numbering and append-only migration files.
- Existing upgrade tests: none. Migration CI validates file discipline and fresh test environments, not a retained-volume release upgrade.
- Existing automated upgrade orchestration: 0 LOC; README documents a two-command manual update procedure.

## Validation prototype

The dedicated Compose file retains the supported app + PostgreSQL topology and project-scoped database volume. It maps the FastAPI port only for deterministic validation and omits the production restart policies and unnecessary host PostgreSQL port. No writable bind or safety relaxation is used.

The seed hook registers a real user through SleepLab's `/auth/register` API. After upgrading only the app service, the verify hook:

1. authenticates that user through `/auth/login`;
2. confirms exactly one matching row in persistent PostgreSQL;
3. checks both target migration filenames in `schema_migrations`; and
4. checks representative target columns in `information_schema`.

LOC uses nonblank, non-comment physical lines:

| Component | LOC |
| --- | ---: |
| Original automated upgrade orchestration | 0 |
| UpgradeProof-specific orchestration (`compose.upgradeproof.yml` + `upgradeproof.yml`) | 50 |
| Project-specific seed | 8 |
| Project-specific verify | 21 |

Integration wall-clock effort was approximately 3 minutes 39 seconds (2026-08-28 22:28:07 to 22:31:46 +08:00), including audit, release/image selection, prototype construction, and execution. The passing path itself took 70.515 seconds, including first-time image pulls.

## Executed evidence

Passing run: `20260828t143023-73fc0db70c26`

- Overall/path status: passed / passed
- Source health, seed, target upgrade, target health, evidence capture, registry digest resolution, verify, report, and scoped cleanup: all passed
- Invariant: API-created user persisted in PostgreSQL and remained login-capable
- Target migration assertions: ledger contains 022/023 and representative adherence columns exist
- JSON and JUnit evidence: `D:\Projects\UpgradeProof-validation\evidence\sleeplab\20260828t143023-73fc0db70c26`

## Abstraction assessment

- Succeeded without modifying UpgradeProof core: **yes**.
- Product limitation: none blocking for an app-only replacement over a stable database service.
- Safety limitation: none. The advanced optional CPAP host mount is read-only and compatible with the current safety contract; it was unnecessary for this invariant.
- Project limitation: no formal version compatibility matrix and no retained-state upgrade CI.
- Environment limitation: none in the passing run.
- New core concept required: **none**.

This result validates one pinned compatibility path. It is not evidence of general production readiness, battle testing, or upstream adoption.
