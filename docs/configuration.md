# Configuration reference

UpgradeProof v0.1.x freezes the public configuration shape at `version: 2`. YAML is decoded strictly: unknown fields, multiple YAML documents, missing required values, duplicate path/check names, and invalid durations are rejected during preflight.

## Complete shape

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
        services:
          - api
          - worker
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

## Top-level fields

- `version` is required and must equal `2`.
- `compose.file` is required and resolves relative to the configuration file. It must name an existing file.
- `paths` requires at least one path. Path names must be non-empty and unique.
- `health.type` must be `http`; `health.url` must be an absolute HTTP(S) URL; timeout and interval must be positive Go duration strings.
- `seed.command` is required and its timeout must be positive.
- `verify.checks` requires at least one check. Check names must be non-empty and unique; every command and positive timeout is required.

## Release states

`from`, every `via` entry, and `to` each describe the environment used to resolve and apply the complete Compose project for that release. Every state must contain a non-empty string-to-string `env` mapping. Environment keys cannot be empty, contain `=`, or use the reserved `UPGRADEPROOF_` prefix (case-insensitive). Values cannot be empty.

`via` is optional and ordered. No service-selection or migration keyword exists: interpolation, dependency ordering, one-shot migrations, profiles, and change-based service recreation use Docker Compose semantics.

## Optional local target build

Only `to` may contain `build`:

- `build.services` is a non-empty, duplicate-free list of Compose service names.
- `build.tag_env` is required and must name a variable declared in the same `to.env` mapping.
- Every listed service must exist in the resolved target model, and its image reference must use `tag_env` as its tag.

At runtime UpgradeProof replaces that environment value with a unique `upgradeproof-target-<run-id>` tag, builds only the listed services, and removes the distinct exact tag references after project cleanup. A registry-backed target simply omits `build`.

## Hook environment

`health`, `seed`, and `verify` are global configuration. The same HTTP health endpoint is awaited after every applied state. Seed runs once per path after `from` is healthy; every verify check runs once per path after the final `to` is healthy.

Hooks inherit the caller environment, then release-state variables, then these tool-controlled variables:

- `UPGRADEPROOF_RUN_ID`
- `UPGRADEPROOF_PROJECT`
- `UPGRADEPROOF_PHASE`
- `UPGRADEPROOF_PATH`
- `UPGRADEPROOF_FROM_STEP`
- `UPGRADEPROOF_CURRENT_STEP`
- `UPGRADEPROOF_TARGET_STEP`
- `UPGRADEPROOF_COMPOSE_FILE`
- `UPGRADEPROOF_REPORT_DIR`

Caller-provided `UPGRADEPROOF_*` values cannot override tool values. UpgradeProof does not serialize complete inherited environments into report metadata, but hook-authored stdout and stderr are retained and must not print secrets.

## Path selection and reports

`upgradeproof test --path NAME` executes one named path; without it, paths execute sequentially. `--report-dir` is interpreted relative to the configuration directory unless absolute. `--keep-on-failure` keeps the failed generated project and any already-built exact run-owned target image tags for investigation.
