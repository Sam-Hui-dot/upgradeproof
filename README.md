# UpgradeProof

**Status: Experimental / Validation Phase**

UpgradeProof tests stateful Docker Compose upgrade paths rather than fresh installs. A historical Compose release state starts first, repository-owned hooks seed real state, the complete Compose model is converged through every declared release environment, and repository-owned checks assert the invariants that must survive.

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
version: 2
compose:
  file: compose.upgrade.yml
paths:
  - name: v1-via-v2-to-current
    from:
      env:
        APP_TAG: v1
    via:
      - env:
          APP_TAG: v2
    to:
      env:
        APP_TAG: current
      build:
        services: [api, worker]
        tag_env: APP_TAG
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

Each `from`, `via`, and `to` item is a complete release state expressed as a non-empty interpolation environment. Compose files use those variables normally, for example `image: example/api:${APP_TAG}`. UpgradeProof runs full-project `docker compose up -d --remove-orphans` for each state, leaving dependency ordering, `service_completed_successfully`, one-shot services, and change-based recreation to Compose. It does not build a separate dependency graph.

An optional local target declares the smallest additional shape: `to.build.services` identifies Compose services to build and `to.build.tag_env` identifies the release variable that receives a run-owned `upgradeproof-target-<run-id>` tag. Registry targets omit `build`. Automatic release discovery and semantic-version path finding are intentionally absent.

Hooks run from the directory containing the UpgradeProof configuration. They inherit the caller's normal environment, overlay the current release environment, and finally overlay UpgradeProof's controlled `UPGRADEPROOF_*` values. Inherited environment and release variables are never copied wholesale into JSON, JUnit, or report metadata.

## Safety boundary

Before Docker resources are created, UpgradeProof audits the declared file and both resolved Compose models for every release state. It rejects external volumes, explicit top-level volume `name`, writable host binds, custom drivers, non-empty `driver_opts`, and fixed `container_name`. Default volumes and explicit `driver: local` without options are allowed. Read-only binds are allowed. Cleanup is exactly `docker compose ... -p <generated-project> down --volumes --remove-orphans`; cleanup refuses any project name not generated with the `upgradeproof-` prefix. Locally built images are removed only by their exact run-owned tags after project cleanup. UpgradeProof never invokes Docker prune commands.

See [docs/safety.md](docs/safety.md) for the exact contract.

## Mandatory validation fixtures

- `fixtures/file-state`: a multi-hop single-service upgrade using a project-scoped named volume; expected exit `0`.
- `fixtures/broken-upgrade`: a healthy target that violates `state-value-preserved`; expected UpgradeProof exit `1`, while the fixture harness passes only when that failure is detected.
- `fixtures/postgres-compose`: an application service upgraded over the same PostgreSQL service and named database volume; expected exit `0`.

Each fixture has a `run-fixture.sh` harness. CI runs all three sequentially on Ubuntu and preserves their JSON, JUnit, hook output, and Compose logs.

## Limitations

Linux, Docker CLI, and Docker Compose v2 are required. Execution is sequential. One path-level HTTP health check is implemented. Upgrade paths are explicit. Hooks are shell commands. Local builds are not reproducible by definition. UpgradeProof relies on Compose convergence and does not add rollback or a general orchestration DSL. Port allocation, Docker HEALTHCHECK mode, database adapters, automatic invariant generation, release packaging, and hosted services remain deferred.

No external repository has adopted UpgradeProof yet. Passing the internal fixtures is only the gate to begin real OSS validation, not proof of product success.
