# UpgradeProof

**Test upgrades, not just fresh installs.**

UpgradeProof starts a historical Docker Compose release, seeds persistent state, converges the same Compose project to a target release, and verifies the invariants that must survive.

```text
Release v1
    ↓
seed persistent state
    ↓
same Compose project / volumes
    ↓
Release v2
    ↓
verify
```

**Status: Experimental public tool. Current release line: v0.1.x.** Linux is the validated execution environment; macOS and Windows binaries are cross-compiled but are not described as battle-tested.

## GitHub Action quick start

Use the versioned Action in a repository workflow:

```yaml
- uses: Sam-Hui-dot/upgradeproof@v0.1.0
  with:
    config: upgradeproof.yml
```

Optional Action inputs are `path`, `report-directory` (default `.upgradeproof`), and `keep-on-failure` (default `false`). The Action downloads the binary corresponding to its release tag and verifies the archive against the published SHA256 checksum before execution. It does not compile UpgradeProof in the consuming repository and does not use `curl | sh`.

## Configuration

The public UpgradeProof v0.1.x configuration schema is `version: 2`. Breaking schema changes are not planned within v0.1.x.

```yaml
version: 2

compose:
  file: compose.upgrade.yml

paths:
  - name: v1-to-v2
    from:
      env:
        APP_TAG: v1
    to:
      env:
        APP_TAG: v2

health:
  type: http
  url: http://127.0.0.1:18080/health
  timeout: 60s
  interval: 2s

seed:
  command: ./upgrade-tests/seed.sh
  timeout: 60s

verify:
  checks:
    - name: users-preserved
      command: ./upgrade-tests/check-users.sh
      timeout: 30s
```

Each `from`, optional `via`, and `to` item is a complete Compose release state expressed as a non-empty interpolation environment. Compose files use the values normally, for example `image: example/api:${APP_TAG}`. UpgradeProof applies the whole project at every state with `docker compose up -d --remove-orphans`. Compose remains responsible for dependency ordering, `service_completed_successfully`, one-shot services, and change-based recreation; UpgradeProof does not add another lifecycle or dependency DSL.

For a target built from the checkout, `to` may declare the Compose services whose image tag is controlled by one release environment variable:

```yaml
to:
  env:
    APP_TAG: current
  build:
    services:
      - api
      - worker
    tag_env: APP_TAG
```

Registry-backed targets omit `build`. See [Configuration reference](docs/configuration.md) for the complete v0.1 schema and validation rules.

## CLI

```text
upgradeproof validate [-c upgradeproof.yml]
upgradeproof test [-c upgradeproof.yml] [--path name] [--keep-on-failure] [--report-dir .upgradeproof]
upgradeproof version
```

Go users can install the command directly:

```sh
go install github.com/Sam-Hui-dot/upgradeproof/cmd/upgradeproof@v0.1.0
```

Exit codes are part of the v0.1 CLI contract:

```text
0  all selected paths passed
1  an executed path failed health, seed, or verification
2  configuration or safety/preflight failure
3  Docker, Compose, evidence, cleanup, or other infrastructure failure
```

`validate` parses strict YAML, audits the raw Compose source, resolves every declared release state with `docker compose config`, and audits every final model without launching containers.

## Evidence and hooks

Every run writes JSON, JUnit, hook stdout/stderr, Compose logs, and per-release service image identity. Hooks run from the directory containing `upgradeproof.yml`. They inherit the caller's normal environment, receive the current release environment, and finally receive UpgradeProof-controlled `UPGRADEPROOF_*` variables. UpgradeProof does not automatically serialize the inherited environment; hook-authored stdout and stderr are still evidence, so hooks must not print secrets.

Reproducible evidence requires pinned or otherwise stable inputs. Mutable image tags and locally built images are recorded but are not presented as fully reproducible.

## Safety boundary

Before creating Docker resources, UpgradeProof audits all resolved states. It rejects external volumes, explicit top-level volume names, writable host binds, custom volume drivers, non-empty `driver_opts`, and fixed `container_name`. Default volumes and explicit `driver: local` without options are allowed; read-only binds are allowed.

Cleanup is limited to generated `upgradeproof-*` Compose project names. Locally built images are removed only by their exact run-owned tag, with duplicate references removed once. UpgradeProof never runs a Docker prune command. See [Safety model](docs/safety.md).

## Compatibility validation

Validated against upgrade scenarios derived from five real OSS projects:

- Savvy
- SleepLab
- Spliit Cloud
- Labby
- Notifuse

**External adoption: 0 confirmed repositories.** Compatibility validation is not upstream adoption, and UpgradeProof is not claimed to be used by those projects. The structured evidence and limitations are retained in [`docs/validation`](docs/validation/).

## Mandatory fixtures

- Fixture A, `fixtures/file-state`: expected UpgradeProof exit `0`.
- Fixture B, `fixtures/broken-upgrade`: the application upgrade is intentionally broken; the harness passes only when UpgradeProof returns exit `1` and reports the failed invariant.
- Fixture C, `fixtures/postgres-compose`: expected UpgradeProof exit `0` with PostgreSQL state preserved.

CI runs gofmt, vet, unit tests, race tests, resolved-safety regressions, and all three fixtures. A separate release-candidate workflow builds the release matrix, verifies checksums and version metadata, and dogfoods `action.yml` against both the passing and intentionally broken fixtures.

## Current limitations

Execution is sequential, upgrade paths are explicit, health waiting is HTTP-only, and hooks are repository-owned shell commands. Local builds are not reproducible by definition. UpgradeProof relies on Docker Compose convergence and does not implement rollback, release discovery, semantic-version path finding, port allocation, a general orchestration DSL, or hosted services.

See [CHANGELOG.md](CHANGELOG.md), [CONTRIBUTING.md](CONTRIBUTING.md), [SECURITY.md](SECURITY.md), and the [Apache-2.0 license](LICENSE).
