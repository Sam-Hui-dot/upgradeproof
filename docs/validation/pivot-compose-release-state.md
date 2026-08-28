# Compose release-state pivot validation

## Decision

Final recommendation: **GO for the narrowed Compose release-state abstraction**.

The pivot removes the disproven single-service versioning unit. A version is now the complete Compose model resolved under one declared interpolation environment. UpgradeProof asks Compose to converge that whole model and does not add a migration keyword, dependency graph, project-specific service roles, or a general orchestration DSL.

This is compatibility evidence, not adoption. **External adoption remains `0 confirmed repositories`.** No upstream pull request, tag, Release, or package publication was created.

## Breaking schema

```yaml
version: 2
compose:
  file: compose.upgrade.yml
paths:
  - name: old-to-new
    from:
      env:
        APP_TAG: v1
    via:
      - env:
          APP_TAG: v2
    to:
      env:
        APP_TAG: current
health:
  type: http
  url: http://127.0.0.1:8080/health
  timeout: 60s
  interval: 2s
seed: {command: ./upgrade-tests/seed.sh, timeout: 60s}
verify:
  checks:
    - {name: state-preserved, command: ./upgrade-tests/verify.sh, timeout: 60s}
```

An optional local target adds only:

```yaml
to:
  env:
    APP_TAG: current
  build:
    services: [api, worker]
    tag_env: APP_TAG
```

The schema is intentionally breaking while experimental. `compose.service`, `compose.image_env`, scalar image steps, and the single-service `to.image/to.build.service` shape were removed.

## Architecture difference

| Concern | Before | After |
| --- | --- | --- |
| Versioning unit | One selected service/image | Entire resolved Compose release state |
| Release transition | `up --no-deps --force-recreate <service>` | Full-project `up -d --remove-orphans` with step env |
| Change detection | UpgradeProof forces one container | Compose recreates changed services and retains stable ones |
| Dependencies | Selected service bypasses dependencies | Existing Compose `depends_on` semantics apply |
| One-shot migration | Cannot coordinate it with other services | Compose reruns it when its resolved image/config changes and gates dependents through `service_completed_successfully` |
| Evidence | One requested/resolved image | Every materialized service, requested image, resolved digest/image ID, and container ID per release step |
| Local target | One fixed `upgradeproof-target:<run-id>` service | Declared build services share a run-owned tag supplied through one release env variable |
| Safety | Raw file plus first source resolved model | Raw file plus resolved and canonical models for every state in every selected path before any project starts |

The release environment is inherited by repository hooks for ordinary Compose commands, but it is not copied wholesale into reports. UpgradeProof-controlled variables still override caller forgeries.

## Safety adjustment

Regression coverage now enforces:

- unnamed/default local volume: allowed;
- explicit `driver: local` with empty/no `driver_opts`: allowed;
- `driver: local` with non-empty `driver_opts`: rejected;
- other/custom driver: rejected;
- external volume: rejected;
- explicit volume `name`: rejected;
- writable bind: rejected;
- resolved include/extends/interpolation bypasses: rejected for every release state.

## Real OSS revalidation

All paths used the same historical and target releases as the initial validation. Local clones contain only compatibility-validation Compose/config/hook glue; no upstream business code or UpgradeProof project-specific core logic was added.

| Repository | Result | Canonical pivot run | Wall clock | Evidence |
| --- | --- | --- | ---: | --- |
| Savvy | passed; no regression | `20260828t160953-1252fef43893` | 27.22s | `D:\Projects\UpgradeProof-validation\evidence\final-pivot-savvy\20260828t160953-1252fef43893` |
| SleepLab | passed; PostgreSQL state/migrations preserved | `20260828t161026-ab178f69b47f` | 18.43s | `D:\Projects\UpgradeProof-validation\evidence\final-pivot-sleeplab\20260828t161026-ab178f69b47f` |
| Spliit Cloud | passed; coordinated release now expressible | `20260828t161050-4a936b9256fb` | 33.81s | `D:\Projects\UpgradeProof-validation\evidence\final-pivot-spliit-cloud\20260828t161050-4a936b9256fb` |
| Labby dedicated topology | passed | `20260828t161135-80508179ee1f` | 31.68s | `D:\Projects\UpgradeProof-validation\evidence\final-pivot-labby\20260828t161135-80508179ee1f` |
| Notifuse dedicated topology | passed | `20260828t161212-a1db85dbe3e2` | 19.89s | `D:\Projects\UpgradeProof-validation\evidence\final-pivot-notifuse\20260828t161212-a1db85dbe3e2` |

Production safety probes remained intentionally red:

- Labby: rejected only for writable `./config` bind; dedicated topology passed.
- Notifuse: explicit `driver: local` is no longer rejected; the checked-in Compose is still rejected for writable `./data` bind.

## Spliit Cloud acceptance evidence

The prototype has one generic release variable, `SPLIIT_TAG`, interpolated into migrate/API/worker/web images. Hooks only seed and verify product state; they do not perform upgrade orchestration.

In the canonical run:

- PostgreSQL container `534e3a891e2e...` remained identical across source and target.
- MailDev container `06a7da3aec34...` remained identical.
- API changed `2b97bf7b9567... -> 6b805af1ed1b...` and resolved to the target API digest.
- One-shot migrate changed `cc48cfc44b8e... -> f36b405b0d1c...` and resolved to the target migrate digest.
- Web changed `e8a519e9b7a6... -> fadb88420a39...`.
- Worker changed `253395a17886... -> 94c742e79879...`.
- The seeded account remained present.
- Prisma migration `20260824131649_split_presets` was recorded as successfully applied.

This directly demonstrates that full-project Compose convergence retained stable dependencies, recreated all changed release services, reran the changed one-shot migration, and honored the existing completion dependency without UpgradeProof knowing which service was a migrator.

## Verification

Local core verification completed with:

- gofmt check;
- `go vet ./...`;
- `go test ./...`;
- Linux `go test -race ./...`;
- resolved safety-bypass regressions;
- Fixture A passed;
- Fixture B harness passed only because UpgradeProof correctly reported the broken target as failed;
- Fixture C passed.

The five OSS runs and raw reports are local compatibility evidence. They do not establish production readiness, battle testing, or external adoption.
