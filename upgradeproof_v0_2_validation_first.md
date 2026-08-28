# UpgradeProof v0.2 — Validation-First Codex Engineering Task

> Working name: **UpgradeProof**
>
> Tagline: **Test upgrades, not just fresh installs.**
>
> Positioning: **A reusable upgrade-path regression testing framework for stateful Docker / Docker Compose applications.**
>
> Philosophy: **Build 30% → validate on real OSS → then build the rest.**

---

# 0. Why this revision exists

This revision replaces the earlier “build the full v0.1 product first” plan.

The problem is real, but the project can still fail by becoming an elegant wrapper around 80–150 lines of Bash. The immediate goal is therefore not release polish. The immediate goal is to prove that UpgradeProof removes enough repeated orchestration work from real OSS projects to justify existing as a reusable tool.

The project lives or dies on real-world integration quality.

---

# 1. Core thesis

Normal CI answers:

```text
Can the newest version start from a fresh state?
```

UpgradeProof must answer:

```text
Can a real historical release create persistent state,
then upgrade through a supported path to the target version,
while preserving declared invariants?
```

The invariant semantics remain application-defined.

UpgradeProof owns the cross-version orchestration.

Users own the business meaning.

---

# 2. Non-negotiable product boundary

UpgradeProof must NOT try to understand business semantics automatically.

Users define:

```text
what data matters
how to seed it
what must still be true after upgrade
```

UpgradeProof handles:

```text
historical versions
explicit multi-hop upgrade paths
Compose project isolation
persistent-state preservation
health waiting
seed/verify hook execution
failure evidence
safe cleanup
repeatable CI orchestration
```

The product wins only if maintainers can stop re-implementing that orchestration themselves.

---

# 3. Immediate success question

Before polishing the product, answer this:

> Can UpgradeProof replace more than half of the upgrade-orchestration glue in at least 3 real OSS repositories, while leaving only project-specific seed and verification logic?

If the answer is no, stop adding features and re-evaluate the abstraction.

---

# 4. Revised development strategy

Do NOT follow:

```text
full CLI
full reports
multi-platform binaries
release polish
GitHub Action
release
then users
```

Follow:

```text
minimal engine
↓
3 strong fixtures
↓
3–5 real OSS integrations
↓
GO / PIVOT / KILL decision
↓
only then freeze schema
↓
GitHub Action + release polish
```

---

# 5. P0 scope

Required commands:

```text
upgradeproof validate
upgradeproof test
upgradeproof version
```

Deferred:

```text
upgradeproof init
upgradeproof doctor
```

Required runtime:

- Go CLI
- Docker CLI
- Docker Compose v2
- Linux as primary supported environment
- sequential execution
- no LLM dependency
- no SaaS/backend

Required outputs:

- concise terminal output
- JSON report
- JUnit report
- captured logs/evidence

Markdown polish is deferred.

---

# 6. Explicit multi-hop upgrade paths — P0

Do not model every test as `old → target`.

Support explicit paths because real applications may require intermediate releases.

Example:

```yaml
version: 1

compose:
  file: compose.upgrade.yml
  service: app
  image_env: UPGRADEPROOF_IMAGE

paths:
  - name: v1-to-current
    from: ghcr.io/example/app:v1.5.0
    via:
      - ghcr.io/example/app:v2.0.0
      - ghcr.io/example/app:v2.7.0
    to:
      build:
        service: app

  - name: latest-stable-to-current
    from: ghcr.io/example/app:v2.9.0
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

Execute exactly as declared:

```text
from
→ via[0]
→ via[1]
→ ...
→ target
```

Automatic path discovery is out of scope.

Semantic-version graph search is out of scope.

---

# 7. State preservation model

Use one generated Compose project identity for the whole path.

Example:

```text
upgradeproof-myrepo-a1b2c3
```

Within one path:

```text
source release
↓
seed state
↓
recreate application service
↓
intermediate release(s)
↓
target
↓
verify
```

Project-scoped named volumes must survive service/container recreation.

Different paths must use different generated project names.

No path may share state with another path.

---

# 8. Safety model — revised and strict

P0 safety must reject storage constructs whose ownership cannot be proven.

## Hard reject: external volumes

Reject:

```yaml
volumes:
  data:
    external: true
```

## Hard reject: explicitly named volumes

Reject by default:

```yaml
volumes:
  data:
    name: actual-production-data
```

Reason: `name:` is used as-is and is not scoped by the Compose project name.

## Hard reject: writable host bind mounts

Reject writable host paths, both absolute and relative.

Examples:

```yaml
- /srv/app-data:/data
- ./data:/data
```

Read-only binds may be allowed when clearly used for source/config.

## Hard reject: local-driver bind tricks

Reject patterns such as:

```yaml
driver_opts:
  type: none
  o: bind
  device: ...
```

## Hard reject: fixed container names

Reject:

```yaml
container_name: my-app
```

because it breaks Compose project isolation.

## Cleanup ownership

Cleanup must only target resources belonging to the generated Compose project.

Never call:

```text
docker system prune
docker volume prune
docker network prune
```

Never remove arbitrary Docker resources by guessed names.

## Interrupt behavior

On Ctrl+C / SIGTERM:

- record interruption
- attempt bounded project-scoped cleanup
- preserve evidence where practical
- exit non-zero

---

# 9. Reproducibility wording — revised

Do not claim UpgradeProof makes mutable image tags fully deterministic.

Use:

```text
deterministic orchestration
```

Record both:

```text
requested image reference
resolved image identity/digest
```

Example:

```text
requested: ghcr.io/example/app:v1.8
resolved:  ghcr.io/example/app@sha256:...
```

Documentation wording:

> UpgradeProof provides repeatable upgrade-path orchestration. Reproducible evidence depends on pinned or otherwise stable inputs.

Local target builds are not automatically reproducible and must not be described as such.

---

# 10. Compose-native contract

Prefer a dedicated test Compose file:

```text
compose.upgrade.yml
```

Example:

```yaml
services:
  app:
    image: ${UPGRADEPROOF_IMAGE}
    build:
      context: .
    ports:
      - "18080:8080"
    volumes:
      - app-data:/data

volumes:
  app-data:
```

Do not mutate Compose YAML to switch versions.

Set:

```text
UPGRADEPROOF_IMAGE=<step image>
```

for source/intermediate images.

For a local target build, build once where practical and reuse the target image across paths.

---

# 11. Multi-service Compose — P0

At least one mandatory integration fixture must use a real multi-service Compose setup.

Minimum:

```text
app
postgres
```

Must prove:

```text
old app + postgres
↓
seed persistent DB state
↓
upgrade app
↓
same postgres state
↓
target app starts
↓
verify data/invariants
```

The abstraction must not assume the application service owns every persistent volume.

---

# 12. Health model

P0:

```yaml
health:
  type: http
  url: http://127.0.0.1:18080/health
  timeout: 60s
  interval: 2s
```

HTTP health is required.

Docker HEALTHCHECK support is deferred unless trivial after HTTP mode is solid.

---

# 13. Hook contract

Hooks run from the repository root.

Required:

```yaml
seed:
  command: ./upgrade-tests/seed.sh

verify:
  checks:
    - name: users-preserved
      command: ./upgrade-tests/check-users.sh
```

Capture for every hook/check:

- start time
- end time
- duration
- exit code
- stdout
- stderr

Expose only controlled environment variables:

```text
UPGRADEPROOF_RUN_ID
UPGRADEPROOF_PROJECT
UPGRADEPROOF_PHASE
UPGRADEPROOF_PATH
UPGRADEPROOF_FROM_IMAGE
UPGRADEPROOF_CURRENT_IMAGE
UPGRADEPROOF_TARGET_IMAGE
UPGRADEPROOF_COMPOSE_FILE
UPGRADEPROOF_REPORT_DIR
```

Do not dump the full process environment into reports.

---

# 14. Engine stages

Recommended model:

```text
PREFLIGHT
RESOLVE_IMAGES
PREPARE_PROJECT
START_FROM
WAIT_FROM_HEALTH
SEED
CAPTURE_SEEDED_STATE
UPGRADE_STEP_1
WAIT_STEP_1_HEALTH
UPGRADE_STEP_2
WAIT_STEP_2_HEALTH
...
UPGRADE_TARGET
WAIT_TARGET_HEALTH
VERIFY
CAPTURE_EVIDENCE
REPORT
CLEANUP
```

Each stage records:

```text
name
started_at
finished_at
duration
status
error
```

A failed stage stops forward execution while still allowing evidence capture and cleanup.

---

# 15. Exit codes

Recommended stable semantics:

```text
0 = all selected paths passed
1 = one or more paths executed but failed health/verification
2 = config or safety/preflight failure
3 = infrastructure/runtime failure
```

A failed invariant must never return 0.

---

# 16. Report minimum

## JSON

Top-level:

```text
tool_version
run_id
started_at
finished_at
config_path
overall_status
paths[]
```

Each path:

```text
name
status
steps[]
checks[]
requested_images[]
resolved_images[]
project_name
artifact_directory
```

Each check:

```text
name
status
exit_code
duration
stdout_path
stderr_path
```

## JUnit

Failed invariants must appear as CI test failures.

## Logs

At minimum preserve:

```text
compose-source.log
compose-step-*.log
compose-target.log
seed.stdout
seed.stderr
verify-*.stdout
verify-*.stderr
```

Do not inline arbitrarily large container logs into JSON.

---

# 17. CLI behavior

## `upgradeproof validate`

Validate:

- schema version
- compose file existence
- service existence where possible
- image interpolation contract
- path shape
- timeout syntax
- target shape
- unsafe volumes
- fixed container names
- writable binds
- explicit volume names
- external volumes

No containers launched.

## `upgradeproof test`

Examples:

```bash
upgradeproof test
upgradeproof test -c upgradeproof.yml
upgradeproof test --path v1-to-current
upgradeproof test --keep-on-failure
upgradeproof test --report-dir .upgradeproof
```

Avoid extra convenience flags before validation.

## `upgradeproof version`

Print build metadata.

---

# 18. Mandatory fixtures

## Fixture A — happy single-service state upgrade

A minimal stateful app using a project-scoped named volume.

Must prove state survives and verification passes.

UpgradeProof returns 0.

## Fixture B — intentionally broken upgrade

Target deliberately violates one declared invariant.

Must prove:

```text
source starts
seed succeeds
target starts
health can pass
verification fails
UpgradeProof returns documented failure code
evidence names failed invariant
```

The CI test succeeds only if UpgradeProof catches this broken upgrade.

## Fixture C — multi-service app + PostgreSQL

Required.

Must prove:

```text
app:v1 + postgres
↓
seed relational data
↓
app upgrade
↓
same DB state
↓
app:target + postgres
↓
verify
```

This fixture is mandatory before external validation.

---

# 19. Unit test priorities

Cover:

- strict YAML decoding
- invalid paths
- multi-hop ordering
- ambiguous target definitions
- duration parsing
- project-name generation
- source/intermediate/target image env handling
- image identity resolution parsing
- hook timeout
- hook stdout/stderr capture
- HTTP health success
- HTTP health timeout
- stage transitions
- verification failure propagation
- exit-code mapping
- external volume rejection
- explicit `volume.name` rejection
- relative writable bind rejection
- absolute writable bind rejection
- local-driver bind rejection
- fixed `container_name` rejection
- safe cleanup command construction
- JSON report
- JUnit report

Do not optimize for test count.

Optimize for meaningful failure coverage.

---

# 20. CI for validation phase

Required on Ubuntu:

```text
gofmt check
go vet ./...
go test ./...
Docker integration fixture A
Docker integration fixture B
Docker integration fixture C
```

`go test -race ./...` is recommended if practical and stable.

Do not build a complex release matrix yet.

---

# 21. Deliberately deferred features

Do not implement yet:

- `upgradeproof init`
- `upgradeproof doctor`
- Markdown polish
- PR comment bot
- Windows release
- macOS release matrix
- Linux ARM release
- GoReleaser polish
- checksum distribution
- automatic release discovery
- semantic upgrade graph
- rollback testing
- DB adapters
- API diff engine
- AI diagnosis
- automatic invariant generation
- production snapshot import
- dashboard
- SaaS

These become eligible only after real-world validation.

---

# 22. Real-world validation — mandatory before v0.1 release

After fixtures A/B/C are green, stop feature development.

Select 3–5 real public OSS repositories.

Criteria:

```text
Docker or Docker Compose
persistent DB or volume
public historical releases
real upgrade/migration behavior
small/medium maintainer team
not already over-engineered for this exact problem
```

Prefer some projects that already have hand-written upgrade tests so code reduction can be measured.

Also include at least one project with weak/no dedicated upgrade coverage if feasible.

For each candidate record:

```text
repository
stars
compose topology
persistent services
supported upgrade path
existing upgrade-test approach
current orchestration LOC
UpgradeProof integration LOC
project-specific seed LOC
project-specific verify LOC
time to integrate
special-case changes needed in UpgradeProof
result
```

---

# 23. Validation death tests

The project may not declare success merely because its own fixtures pass.

## GO

Strong continue signal:

```text
≥3 real OSS repos successfully integrated
>50% reduction in orchestration glue where existing upgrade tests exist
business checks remain project-specific and understandable
existing-upgrade-test project integrates in roughly ≤1 hour
same schema works across materially different Compose topologies
```

## PIVOT

Pivot if:

```text
projects integrate,
but each needs repeated new lifecycle concepts
or lots of UpgradeProof-specific shell glue
```

## KILL

Seriously consider stopping if:

```text
3 of 5 real projects cannot fit cleanly
4 of 5 require product-specific hacks in core
integration barely saves code over Bash
most remaining work is orchestration rather than business seed/verify
```

Do not rescue a bad abstraction with dashboard, AI, or plugin framework.

---

# 24. The Bash test

For every integration ask:

> Why would this maintainer use UpgradeProof instead of one shell script?

The answer must not be:

```text
because output is prettier
```

It should become:

```text
because UpgradeProof standardizes multi-version/multi-hop execution,
project isolation,
volume ownership safety,
health waiting,
evidence capture,
cleanup,
CI failure semantics,
and reporting,
while the repository keeps only domain-specific seed/assertion logic.
```

If this is not true in practice, the product is not ready.

---

# 25. OSS integration discipline

Do not spam external repos.

Before a real integration:

1. read CONTRIBUTING
2. inspect upgrade docs/tests
3. verify a real gap
4. prototype in fork/local clone
5. measure integration effort
6. keep proposed PR small
7. explain exactly which historical path is tested

Do not claim adoption unless upstream actually accepts or uses it.

A forked demo is compatibility evidence, not adoption.

---

# 26. Schema stability rule

Before real OSS validation:

```text
schema = experimental
```

Breaking changes are allowed.

After 3 successful materially different integrations:

```text
review schema
freeze v1 shape
prepare public v0.1.0
```

Do not freeze a bad schema early.

---

# 27. README during validation

Use honest status:

```text
Status: experimental / validation phase
```

Explain:

- problem
- why fresh-install CI is insufficient
- what users must provide
- what UpgradeProof handles
- limitations
- safety

Do not claim:

```text
production ready
battle tested
widely adopted
industry standard
```

unless later evidence supports it.

---

# 28. Working name and repo

Working name:

```text
UpgradeProof
```

Preferred repo:

```text
Sam-Hui-dot/upgradeproof
```

Before public creation, perform a final exact-name/software-product collision check.

Apache-2.0 remains acceptable.

---

# 29. Suggested Go structure

```text
cmd/upgradeproof/
    main.go

internal/
    config/
    compose/
    engine/
    health/
    hooks/
    report/
    safety/
    imageid/

fixtures/
    file-state/
    broken-upgrade/
    postgres-compose/

docs/
    validation-notes.md
    safety.md

.github/workflows/
README.md
LICENSE
go.mod
```

Docker execution must sit behind a small abstraction.

Do not scatter raw Docker `exec.Command` calls across packages.

---

# 30. First Codex implementation pass

Codex should do ONLY this:

### A. Verify exact Compose semantics

Research current behavior for:

- project-scoped named volumes
- explicit volume `name`
- external volumes
- bind mounts
- project names
- image interpolation
- container recreation with preserved volumes
- `docker compose config` output useful for safety parsing

### B. Build minimal CLI

```text
validate
test
version
```

### C. Implement strict safety preflight

P0.

### D. Implement explicit multi-hop paths

P0.

### E. Implement source/intermediate/target lifecycle

Preserve state across every hop.

### F. Implement HTTP health + seed + verify

### G. Implement evidence + JSON/JUnit

No report-polish marathon.

### H. Build fixtures A/B/C

Especially PostgreSQL multi-service fixture.

### I. Run CI

Everything green.

### J. STOP FEATURE DEVELOPMENT

Do not proceed to full release engineering.

Produce a validation-ready report and move to real OSS integration.

---

# 31. Final report required from Codex

Return:

1. local project path
2. repo URL if created
3. branch
4. commit SHA
5. commands implemented
6. config schema
7. exact multi-hop lifecycle
8. Docker safety checks
9. `volume.name` handling
10. relative/absolute bind handling
11. image digest/effective identity recording
12. fixture A result
13. fixture B result
14. fixture C result
15. unit tests
16. `go vet`
17. `go test`
18. race test if run
19. CI URL/status if available
20. example JSON report
21. example JUnit report
22. known limitations
23. schema compromises discovered
24. final `git status`
25. external adoption count

Expected during this phase:

```text
External adoption: 0 confirmed repositories
```

Do not hide that.

---

# 32. Release prohibition

Do NOT create:

```text
v0.1.0 tag
GitHub Release
package-manager publication
multi-platform public binary release
```

until:

```text
fixtures pass
AND
real-world validation has been attempted
AND
the schema has survived at least 3 materially different integrations
```

Internal/pre-release artifacts are acceptable for testing.

---

# 33. Agent operating instruction

Work continuously and autonomously.

Do not stop after scaffolding.

Do not expand scope beyond this file.

Do not add deferred features because they are “easy.”

Interrupt only for:

- credentials
- missing external permission
- irreversible action with unclear authorization
- genuinely ambiguous product choice not resolved here

When core + fixtures are complete, stop feature expansion and report readiness for real-OSS validation.

---

# 34. Current standard of proof

Do not ask:

```text
Does UpgradeProof look complete?
```

Ask:

```text
Does it remove meaningful repeated upgrade-test orchestration from real OSS projects?
```

That is the standard that decides whether this project deserves to continue.
