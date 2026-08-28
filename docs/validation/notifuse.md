# Notifuse compatibility validation

## Outcome

**Compatibility validated:** yes for a dedicated project-scoped named-volume topology; **not validated against the repository Compose file unchanged**.

**External adoption:** no. This is a local compatibility prototype only; upstream has not accepted or adopted UpgradeProof.

No UpgradeProof core code or Notifuse business code was changed. Only validation Compose/config/seed/verify glue was added in the local clone.

## Repository and release evidence

| Field | Result |
| --- | --- |
| Repository | `Notifuse/notifuse` (`https://github.com/Notifuse/notifuse`) |
| Stars | 2,085, GitHub API snapshot at 2026-08-28 22:52 +08:00 |
| Audited revision | `63d70e5d667fe89381d439e5fd4016038ae3b73b` (`main`, also tag `v38.0`) |
| Historical release | `v37.2`, published 2026-08-06 |
| Target release | `v38.0`, published 2026-08-17 |
| Source image | `notifuse/notifuse@sha256:7c9eec2b7b126c0b78f3485ccd8ca85645dceb54c464cb28f0ecc3c2d678d4ac` |
| Target image | `notifuse/notifuse@sha256:ac111974bce19b60ac243de5cdf26ff108dee523ad41406f265b3fd40d3c4414` |
| Official deployment guidance | Repository Compose is recommended for testing/development; standalone published image with an external PostgreSQL 17+ database is the production option |
| Realistic upgrade path | Replace the published application image while retaining PostgreSQL and the unchanged `SECRET_KEY`; migrations run at application startup |

The release pair is meaningful: v38 adds a system permission backfill and per-workspace annotations, usage, and web-analytics schema/trigger migrations. The repository has no `CONTRIBUTING` file. README explicitly states that unsolicited pull requests are not accepted, which reinforces that this work is local compatibility validation only.

## Architecture audit

- Checked-in Compose topology: locally built API plus PostgreSQL 17 on a bridge network. The API exposes HTTP and optional SMTP bridge ports. An AlloyDB Compose variant replaces local PostgreSQL with a cloud connector.
- Persistent resources: `./data:/app/data` writable host bind for API data/cert material and a PostgreSQL named volume declared with explicit `driver: local`. Production standalone deployments use an externally managed PostgreSQL server; each workspace receives its own database.
- Migration mechanism: startup reads `settings.db_version` from the system database, runs registered major migrations transactionally, applies workspace portions to every workspace database, advances the system version, and can request a process restart. v38 does not require a restart.
- Existing CI: Go 1.25 unit tests with race detection; PostgreSQL 17 + Mailpit integration tests with `go test -race -tags integration`; console, web analytics SDK, image build/release, and review workflows.
- Existing migration tests: manager tests plus 547 LOC of targeted v38 unit tests covering registration, system updates, workspace DDL, trigger repair, rollback, and error paths.
- Existing full release-upgrade test: none. CI does not boot an old published image, retain its databases, then replace it with a new release image.
- Existing automated release-upgrade orchestration: 0 LOC.

## Repository Compose safety result

Running UpgradeProof validation against a minimal probe preserving the repository's persistence declarations was rejected before container creation:

```text
unsafe Compose configuration:
service "api" image must interpolate UPGRADEPROOF_IMAGE
service "api" volume[0]: writable bind mount source "./data"
volume "postgres-data" uses custom volume driver "local"; ownership cannot be proven
```

The writable bind rejection is a reasonable non-destructive boundary because UpgradeProof cannot prove ownership of arbitrary host data. The explicit `driver: local` rejection is the validation-phase fail-closed policy: although Docker's default local driver is common, an explicit custom-driver declaration is outside the presently proven cleanup contract. Both are practical blockers to zero-change use of the checked-in Compose file; safety was not disabled to make the project pass.

## Dedicated validation prototype

The dedicated topology uses the official published API images plus PostgreSQL 17. It changes the API bind and PostgreSQL declaration to ordinary project-scoped named volumes and removes the explicit driver. It also omits the host PostgreSQL/SMTP ports and local image build. Therefore it is representative of application + persistent PostgreSQL behavior, but is not the checked-in deployment topology unchanged.

The seed hook uses real public application interfaces:

1. completes the Setup Wizard to create the root user;
2. waits for the documented post-setup restart;
3. authenticates through root HMAC sign-in;
4. creates a real workspace through `/api/workspaces.create`; and
5. confirms the workspace has a real contact and lacks target-only web analytics tables on v37.2.

After replacement with v38.0, verify confirms root-user preservation, system `db_version=38`, workspace/contact preservation, and creation of `annotations`, `web_sessions`, `web_pages`, and `web_goals` in the workspace database.

LOC uses nonblank, non-comment physical lines:

| Component | LOC |
| --- | ---: |
| Original automated release-upgrade orchestration | 0 |
| Existing targeted v38 migration tests | 547 |
| UpgradeProof-specific orchestration (`compose.upgradeproof.yml` + `upgradeproof.yml`) | 70 |
| Production-safety probe config (reported separately) | 23 |
| Project-specific seed | 59 |
| Project-specific verify | 16 |

Integration wall-clock effort was approximately 12 minutes (2026-08-28 22:45:19 to 22:57:21 +08:00), including audit, official-document and registry verification, production safety rejection, prototype construction, diagnostic iterations, and execution. The final cached run took 17.20 seconds.

## Executed evidence

Canonical passing run: `20260828t145651-d26a0177e700`

- Overall/path status: passed / passed
- Source/target health, seed, image resolution, retained PostgreSQL, migrations, verify, reports, and scoped cleanup: all passed
- Source invariant: real root user, workspace, and owner contact created on v37.2; target-only workspace tables absent
- Target assertions: root/workspace/contact retained; system version 38; v38 workspace analytics and annotation tables present
- JSON and JUnit evidence: `D:\Projects\UpgradeProof-validation\evidence\notifuse\20260828t145651-d26a0177e700`

Diagnostic evidence was retained:

1. `20260828t145142-38085b27978f` passed root-user/system-version preservation but did not create a workspace; it was superseded as insufficiently deep migration coverage.
2. `20260828t145415-6a3546d190f7` exposed setup-restart timing and an unavailable host `jq`; classification: validation glue/environment assumption.
3. `20260828t145506-0be2b944d2ee` exposed an incorrect prototype workspace-database name; classification: validation glue assumption.
4. `20260828t145550-68c28d299a97` exposed an incorrect assumption that workspace databases carry the system version ledger; classification: validation assertion error.

None required or justified an UpgradeProof core change.

## Abstraction assessment

- Succeeded without modifying UpgradeProof core: **yes for the dedicated topology; no direct run against the checked-in Compose**.
- Product limitation: none blocking for one application image over a stable PostgreSQL service.
- Safety limitation: writable bind and explicit local driver are rejected under the current ownership contract.
- Project limitation: no retained-state release-upgrade CI or formal version compatibility matrix; the repository Compose is documented for testing rather than production.
- Environment limitation: `jq` was absent, so final glue uses POSIX tools; this did not affect the canonical result.
- New core concept required: none for the dedicated compatibility path. Supporting explicit default-local volumes or external production databases would require a proven ownership/non-cleanup contract, not a Notifuse exception.

This validates one pinned compatibility path and a deliberately different safe topology. It does not establish upstream adoption or unchanged compatibility with Notifuse's checked-in Compose.
