# Savvy compatibility validation

## Outcome

**Compatibility validated:** yes, for the explicitly tested `v1.2.2 -> v1.2.3` image path.

**External adoption:** no. This is a local compatibility prototype only; upstream has not accepted or adopted UpgradeProof.

No UpgradeProof core code or Savvy business code was changed. The prototype consists only of a validation Compose file, UpgradeProof config, seed hook, and verify hook in the local validation clone.

## Repository and release evidence

| Field | Result |
| --- | --- |
| Repository | `truenormis/savvy` (`https://github.com/truenormis/savvy`) |
| Stars | 44, GitHub API snapshot at 2026-08-28 22:27 +08:00 |
| Audited revision | `1ce0ae6339e7c71f1a2ad47f379d8dac33bca04e` (`main`, also tag `v1.2.3`) |
| Historical release | `v1.2.2`, published 2026-01-11 |
| Target release | `v1.2.3`, published 2026-05-28 |
| Source image | `truenormis/savvy@sha256:dc71766fd4cc5655a8ed5cc5067e3bf685334912813a38c41ab6c505cbb40ea9` |
| Target image | `truenormis/savvy@sha256:871261951b039e7211cb274877fc3fa69f984bc54d9107594ddd5c0b78391564` |
| Upstream upgrade guidance | `docker compose pull` followed by `docker compose up -d`; no compatibility matrix or release-to-release guarantee |
| Realistic supported path | Direct replacement while retaining `/data`; `v1.2.2 -> v1.2.3` is realistic and both are non-prerelease releases with published images |

The repository has no `CONTRIBUTING` file. README contribution guidance asks contributors to open an issue before a pull request.

## Architecture audit

- Compose topology: one application container. The image uses Supervisor to run nginx, PHP-FPM, the Laravel scheduler, and (in `v1.2.3`) a queue worker in one container.
- Persistent resources: a project-scoped named volume mounted at `/data`, containing SQLite state and generated environment configuration. SQLite WAL is enabled.
- Migration mechanism: the entrypoint creates and seeds a fresh database once, then runs `php artisan migrate --force` on every startup before starting services.
- Upgrade delta: `v1.2.3` adds the `jobs` and `failed_jobs` migrations, plus container hardening and health/readiness endpoints.
- Existing tests: Pest unit/feature tests, a fresh-image Docker build plus goss smoke contract, Dockerfile lint, and Trivy. There is no existing stateful or release-to-release upgrade test.
- Existing upgrade orchestration: 0 LOC automated; README documents a two-command manual update procedure.

## Validation prototype

The dedicated Compose file preserves the real one-container plus `/data` named-volume topology and adds a validation-only health bridge. It deliberately omits the README quick-start example's fixed `container_name`, which UpgradeProof correctly rejects; the repository's checked-in Compose file also has no fixed name.

The bridge was necessary because `v1.2.2` exposes Laravel's `/up` health route while `v1.2.3` replaces it with `/livez` and `/readyz`. There is no common reliable application HTTP health URL across the releases. The validation-only sidecar reports HTTP success only after it can establish a TCP connection to the app's port 80. It owns no persistent state and is not upgraded.

The seed hook inserts a real row in Savvy's stable `users` table. The verify hook proves that row survives and that the two target-only queue tables exist after migration.

LOC uses nonblank, non-comment physical lines:

| Component | LOC |
| --- | ---: |
| Original automated upgrade orchestration | 0 |
| UpgradeProof-specific orchestration (`compose.upgradeproof.yml` + `upgradeproof.yml`) | 54 |
| Project-specific seed | 10 |
| Project-specific verify | 22 |

Integration wall-clock effort was approximately 11 minutes 37 seconds (2026-08-28 22:15:47 to 22:27:24 +08:00), including repository audit, image verification, prototype construction, two diagnostic failures, and the passing run. The final upgrade run itself took 22.074 seconds.

## Executed evidence

Passing run: `20260828t142631-cb3310b0f801`

- Overall/path status: passed / passed
- Source health, seed, target upgrade, target health, evidence capture, image resolution, verify, report, and scoped cleanup: all passed
- Invariant: seeded `upgradeproof@example.invalid` user preserved
- Target migration assertions: `jobs` and `failed_jobs` tables present
- JSON and JUnit evidence: `D:\Projects\UpgradeProof-validation\evidence\savvy\20260828t142631-cb3310b0f801`

Diagnostic evidence was retained:

1. `20260828t142016-a019811559de` failed before seed because the initial prototype assumed a common root HTTP endpoint. Classification: validation glue error exposing a health-model integration cost.
2. `20260828t142517-364dbac555b4` completed the upgrade and preserved the user but the prototype incorrectly asserted an upstream `job_batches` table that does not exist. Classification: validation assertion error.

Neither diagnostic failure required or justified a core change.

## Abstraction assessment

- Succeeded without modifying UpgradeProof core: **yes**.
- Product limitation: no blocking limitation for this path. A single fixed HTTP health URL creates glue when historical releases rename health endpoints; command/TCP health could reduce integration LOC, but it is not required to express or validate the upgrade.
- Safety limitation: the documented fixed `container_name` quick-start cannot be used unchanged. This is a reasonable boundary because a project-scoped equivalent exists and matches the checked-in Compose topology.
- Project limitation: no formal release-to-release compatibility promise and no existing upgrade test.
- Environment limitation: none in the passing run.
- New core concept required: **none**.

This result establishes compatibility for one pinned path. It does not establish general Savvy compatibility, production readiness, battle testing, or upstream adoption.
