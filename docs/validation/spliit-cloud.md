# Spliit Cloud compatibility validation

## Outcome

**Compatibility validated:** no, not for the supported coordinated production topology.

**External adoption:** no. This is a local compatibility prototype only; upstream has not accepted or adopted UpgradeProof.

The run intentionally remains red. UpgradeProof upgraded and resolved the API image, but its current single-versioned-service abstraction did not rerun the target migration container or replace the worker and web containers. No hook repaired or hid that mismatch.

No UpgradeProof core code or Spliit Cloud business code was changed. Only validation Compose/config/seed/verify glue was added in the local clone.

## Repository and release evidence

| Field | Result |
| --- | --- |
| Repository | `antonio-ivanovski/spliit-cloud` (`https://github.com/antonio-ivanovski/spliit-cloud`) |
| Stars | 51, GitHub API snapshot at 2026-08-28 22:32 +08:00 |
| Audited revision | `3b8ab306aa25013a199a533ee5fdeb6520bf4228` (`main`, immutable rolling release available) |
| Historical release | `sha-5c9abdc6cdaf6eb32c8ea01f4882ac5b0e670380`, published 2026-08-23 |
| Target release | `sha-f1a8254898e692bbeb60989e035ec0629bab8e85`, published 2026-08-25 |
| Upgrade delta | Target adds the complex `20260824131649_split_presets` Prisma data/schema migration plus matching API and web behavior |
| Official upgrade path | Pin one immutable `SPLIIT_TAG`, back up PostgreSQL, `docker compose ... pull`, then `docker compose ... up -d` |
| Rollback guidance | Restore the old tag; restore the matching database backup if a migration is incompatible |

Each release publishes separate API, migrate, worker, and web images under one commit tag. Registry manifest digests were verified before execution:

| Component | Source digest | Target digest |
| --- | --- | --- |
| api | `sha256:d4a7294546b444e7430796ba436eec20578ac60ea435c6eb29edad652d88de19` | `sha256:7df5a353ddec6de2807fd9774684d24fea0c441e11997dc81ea8492be55c03f4` |
| migrate | `sha256:719833c1eeb33d8de51122cd5aea286e538f7a3ae96529bb26e872a9f609efcc` | `sha256:36d6b9dd97acc25764e2e1ba66dff88c1b3d267b1d23620a2bf112ee07026358` |
| worker | `sha256:bbad782f6aca882228c0ebc361181b9184024516a8f3c113016f77496b748515` | `sha256:61373da8ea5929017d3c403d96369f02fbc6b2f377cf453af73dade75099795d` |
| web | `sha256:dcb985ad9fd1a0867de8c950557306635ad75a23003333ea137186bbd0cfd7ae` | `sha256:8db4a911e5b9bb0dd2a5f11ccb422772f326a644e085cd1c2ead01b8abf6c67d` |

## Architecture audit

- Supported Compose topology: PostgreSQL, one-shot `migrate`, API, background worker, and web gateway. SMTP is required; optional object storage, MCP, and AI integrations are outside this prototype.
- Persistent resources: the `postgres_data` named volume is primary state. Optional expense-document object storage is separately persistent in deployments that enable it.
- Migration mechanism: a release-matched migrate image runs `prisma migrate deploy`; API and worker depend on successful completion, and web depends on API.
- Release mechanism: successful CI builds five component images for amd64/arm64, publishes immutable full-commit tags plus moving `latest`, and creates SHA/rolling releases.
- Existing tests: unit and real-PostgreSQL integration tests, Compose config validation, build/type/lint/i18n checks, and migration-specific replay tests. The chosen split-preset migration has a 278-LOC targeted test that replays the historical schema, seeds legacy rows, applies SQL with deployment-like statement commits, and checks transformed data.
- Existing full release-upgrade test: none. The migration harness does not start old component images and then coordinate replacement with new ones.
- Existing automated full release-upgrade orchestration: 0 LOC. Targeted chosen-migration harness: 278 LOC (reported separately because it is not release orchestration).

## Validation prototype

The prototype preserves the essential PostgreSQL + migrate + API + worker + web topology and adds MailDev only to support a real signup. All four versioned application images interpolate the same immutable release tag. `compose.service` is `api`, because UpgradeProof currently permits exactly one selected versioned service.

The seed hook creates a real account through `/auth/sign-up/email`. The first verify check confirms the account persists. The second checks every component's actual container image and Prisma's `_prisma_migrations` ledger.

LOC uses nonblank, non-comment physical lines:

| Component | LOC |
| --- | ---: |
| Original automated full release-upgrade orchestration | 0 |
| Existing targeted split-preset migration harness | 278 |
| UpgradeProof-specific orchestration (`compose.upgradeproof.yml` + `upgradeproof.yml`) | 93 |
| Project-specific seed | 8 |
| Project-specific verify (both checks) | 31 |

Integration wall-clock effort was approximately 7 minutes 17 seconds (2026-08-28 22:32:33 to 22:39:50 +08:00), including audit, eight-image manifest verification, prototype construction, and two confirming runs. The final cached run took 19.257 seconds.

## Executed evidence

Canonical run: `20260828t143901-78f947b77dd0`

- Overall/path status: failed / failed — this is the expected and correct validation result
- Source stack startup, source/target health, seed, account-state invariant, evidence capture, image resolution, report, and scoped cleanup: passed
- `account-state-preserved`: passed
- API actual image: target release
- migrate actual image: source release
- worker actual image: source release
- web actual image: source release
- target `20260824131649_split_presets` migration: absent
- `coordinated-release-applied`: failed
- JSON and JUnit evidence: `D:\Projects\UpgradeProof-validation\evidence\spliit-cloud\20260828t143901-78f947b77dd0`

The earlier confirming run `20260828t143634-4bd6f7e07b93` reached the same result; the canonical rerun only removed harmless Compose interpolation warnings from the validation hook output.

## Abstraction assessment

- Succeeded without modifying UpgradeProof core: **no**, for the supported production upgrade path.
- Failure classification: **product limitation**.
- Why: `UpgradeProof` sets one version value but executes `docker compose up --no-deps --force-recreate <one service>`. A Spliit Cloud release is a coordinated set of images with an ordered one-shot migration. Selecting API cannot update or rerun migrate/worker/web; selecting migrate cannot replace and health-check the long-lived services.
- Safety limitation: none encountered by the dedicated representative Compose.
- Project limitation: none responsible for this failure; the project publishes immutable coordinated images and documents their order.
- Environment limitation: none.
- New core concept needed: a **coordinated release/service group** that can map one release identifier to multiple service images, run ordered one-shot migration services, recreate dependent long-lived services, and assert health/version across the group.

That concept is evidence-backed but was not implemented in this phase. Implementing it ad hoc for Spliit Cloud would violate the no-hardcoding/no-premature-core-expansion constraint.

