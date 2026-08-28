# UpgradeProof

**Status: Experimental / Validation Phase**

UpgradeProof tests stateful Docker Compose upgrade paths rather than fresh installs. A historical release starts first, repository-owned hooks seed real state, the selected application service is recreated through every declared intermediate image and target, and repository-owned checks assert the invariants that must survive.

UpgradeProof owns deterministic orchestration: unique Compose projects, ordered version hops, HTTP health waiting, evidence capture, safe project-scoped cleanup, CI exit semantics, JSON, and JUnit. The application repository still owns the business meaning of its seed and verification hooks.

Reproducible evidence depends on pinned or otherwise stable inputs. Mutable image tags and local builds are recorded but are not described as fully reproducible.

## Commands

```text
upgradeproof validate [-c upgradeproof.yml]
upgradeproof test [-c upgradeproof.yml] [--path name] [--keep-on-failure] [--report-dir .upgradeproof]
upgradeproof version
```

Exit codes are stable in this experimental implementation:

```text
0  all selected paths passed
1  an executed path failed health, seed, or verification
2  configuration or static safety/preflight failure
3  Docker, Compose, evidence, cleanup, or other infrastructure failure
```

`validate` parses strict YAML, checks the P0 schema and static Compose safety contract, and runs `docker compose config --format json` without launching containers.

## Experimental configuration

```yaml
version: 1
compose:
  file: compose.upgrade.yml
  service: app
  image_env: UPGRADEPROOF_IMAGE
paths:
  - name: v1-via-v2-to-current
    from: example/app:v1
    via:
      - example/app:v2
    to:
      build:
        service: app
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

`to` must define exactly one of `image` or `build`. Local targets use `to.build.service`; registry targets use `to.image`. Automatic release discovery and semantic-version path finding are intentionally absent.

The application service image in the Compose file must interpolate `${UPGRADEPROOF_IMAGE}`. Hooks run from the directory containing the UpgradeProof configuration and receive only the documented `UPGRADEPROOF_*` variables.

## Safety boundary

Before Docker resources are created, UpgradeProof rejects external volumes, explicit top-level volume `name`, writable relative or absolute host binds, local-driver bind tricks, and fixed `container_name`. Read-only binds are allowed. Cleanup is exactly `docker compose ... -p <generated-project> down --volumes --remove-orphans`; cleanup refuses any project name not generated with the `upgradeproof-` prefix. UpgradeProof never invokes Docker prune commands.

See [docs/safety.md](docs/safety.md) for the exact contract.

## Mandatory validation fixtures

- `fixtures/file-state`: a multi-hop single-service upgrade using a project-scoped named volume; expected exit `0`.
- `fixtures/broken-upgrade`: a healthy target that violates `state-value-preserved`; expected UpgradeProof exit `1`, while the fixture harness passes only when that failure is detected.
- `fixtures/postgres-compose`: an application service upgraded over the same PostgreSQL service and named database volume; expected exit `0`.

Each fixture has a `run-fixture.sh` harness. CI runs all three sequentially on Ubuntu and preserves their JSON, JUnit, hook output, and Compose logs.

## Limitations

Linux, Docker CLI, and Docker Compose v2 are required. Execution is sequential. Only HTTP health checks are implemented. Upgrade paths are explicit. Hooks are shell commands. Local builds are not reproducible by definition. Port allocation, Docker HEALTHCHECK mode, rollback, database adapters, automatic invariant generation, release packaging, and hosted services are deferred until real OSS validation proves the abstraction.

No external repository has adopted UpgradeProof yet. Passing the internal fixtures is only the gate to begin real OSS validation, not proof of product success.
