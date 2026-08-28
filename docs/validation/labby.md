# Labby compatibility validation

## Outcome

**Compatibility validated:** yes for a dedicated named-volume validation topology; **not validated for the recommended production writable-bind topology**.

**External adoption:** no. This is a local compatibility prototype only; upstream has not accepted or adopted UpgradeProof.

No UpgradeProof core code or Labby business code was changed. Validation-only Compose/config/seed/verify glue was added in the local clone.

## Repository and release evidence

| Field | Result |
| --- | --- |
| Repository | `samuelloranger/labby` (`https://github.com/samuelloranger/labby`) |
| Stars | 87, GitHub API snapshot at 2026-08-28 22:41 +08:00 |
| Audited revision | `683d10f0a9b4acdceb2e0ef90e78f6827e99a1f5` (`main`) |
| Historical release | `v1.3.0`, published 2026-06-19 |
| Target release | `v1.4.0`, published 2026-06-27 |
| Source image | `ghcr.io/samuelloranger/labby@sha256:0204eba59d6420229df5bafb812b83358a2445683abf66246ad6614da1147b14` |
| Target image | `ghcr.io/samuelloranger/labby@sha256:208cf59c45920b3f10e39f9a75f63140f8f9296abbbf3a559019abe7a6bbd7d9` |
| Upstream deployment | One non-root container with `./config:/app/config`; README requires that host directory to be writable by the container user |
| Realistic upgrade path | Replace/pull the image while retaining `config/labby.db`; no explicit upgrade guide or compatibility matrix |

`v1.3.0 -> v1.4.0` was selected because it adds the integration-driven layout and a real SQLite migration (`5_integration_position`) plus a guarded transactional layout-data migration. The repository has no `CONTRIBUTING` file.

## Architecture audit

- Compose topology: one Bun application container.
- Persistent resources: production uses a writable relative host bind at `./config`, containing `labby.db` and plaintext backups (including credentials).
- Migration mechanism: ordered built-in SQLite migrations run on module load inside one transaction and are recorded in `_migrations`. A separate guarded transaction migrates dashboard layout data into integrations and writes `layout_migrated=1`.
- Existing tests: Bun unit tests, typecheck, Biome lint, web/server builds, optional local Playwright E2E, and a 71-LOC targeted layout-migration unit test.
- Existing release-upgrade tests: none. Tests use temporary/fresh SQLite state rather than old and new published containers over the same storage.
- Existing automated release-upgrade orchestration: 0 LOC.

## Production safety result

Running UpgradeProof validation directly against the checked-in `docker-compose.yml` was rejected before any container creation:

```text
unsafe Compose configuration:
service "labby" image must interpolate UPGRADEPROOF_IMAGE
service "labby" volume[0]: writable bind mount source "./config"
```

The bind rejection is a reasonable safety boundary: UpgradeProof cannot prove ownership of or safely delete an arbitrary host directory that contains user credentials and backups. It is also a practical adoption blocker for a zero-change integration because this is Labby's only documented production persistence topology.

## Dedicated validation prototype

The dedicated topology replaces `./config` with a run-owned project-scoped named volume. A validation-only Alpine init service grants write access to that empty volume, after which Labby still runs as its image's default non-root user. The init step owns no application behavior and is not part of production Labby.

This differs materially from production:

- data is not directly visible at `./config` on the host;
- host filesystem ownership guidance no longer applies;
- backup discovery/copy procedures would differ; and
- the temporary init step is extra orchestration.

The seed hook creates a real monitor integration via Labby's API. After upgrading, verify checks API-visible preservation, SQLite migration v5, the new `position` value, and the guarded `layout_migrated` flag.

LOC uses nonblank, non-comment physical lines:

| Component | LOC |
| --- | ---: |
| Original automated release-upgrade orchestration | 0 |
| UpgradeProof-specific orchestration (`compose.upgradeproof.yml` + `upgradeproof.yml`) | 41 |
| Production-safety probe config (reported separately) | 23 |
| Project-specific seed | 7 |
| Project-specific verify | 15 |

Integration wall-clock effort was approximately 3 minutes 20 seconds (2026-08-28 22:41:20 to 22:44:40 +08:00), including audit, production rejection, dedicated prototype, and execution. The passing path took 44.501 seconds, including first-time image pulls.

## Executed evidence

Passing dedicated-topology run: `20260828t144339-b73852eea00f`

- Overall/path status: passed / passed
- Source/target health, seed, image resolution, state/migration verify, report, and scoped cleanup: all passed
- Invariant: API-created `UpgradeProof Marker` integration persisted
- Target assertions: migration v5 recorded, integration `position` populated, `layout_migrated=1`
- JSON and JUnit evidence: `D:\Projects\UpgradeProof-validation\evidence\labby\20260828t144339-b73852eea00f`

Production safety probe: validation exit was nonzero and no containers were created.

## Abstraction assessment

- Succeeded without modifying UpgradeProof core: **yes for dedicated named-volume compatibility validation; no for direct use of production Compose**.
- Product limitation: none for the single-container upgrade itself.
- Safety limitation: writable host state cannot enter the current ownership/cleanup contract.
- Project limitation: the image's non-root user needs writable storage, so an empty named volume needs permission initialization; the project documents only host-bind persistence.
- Environment limitation: none.
- New core concept required: none to validate the named-volume path. Supporting production bind semantics would require an explicitly different non-destructive external-state contract, not a Labby-specific exception.

The passing result must not be represented as proof that Labby's production topology is compatible. It establishes only the clearly documented named-volume compatibility prototype.
