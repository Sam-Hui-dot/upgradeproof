# Validation-phase notes

## Current decision state

The schema is experimental. The core engine and the three mandatory local fixtures are the only implementation target before real OSS work. No public `v0.1.0` tag, GitHub Release, package publication, or multi-platform release is permitted at this stage.

External adoption: **0 confirmed repositories**.

## Compose semantics used by the engine

- `docker compose -p` overrides environment, top-level `name`, and directory-derived project names.
- A normal named volume is project-scoped; explicit `volume.name` is used as-is, and an external volume is managed outside the Compose application.
- `docker compose up` recreates a service when its image/config changes while preserving mounted volumes unless anonymous volumes are explicitly renewed.
- Compose interpolation accepts `UPGRADEPROOF_IMAGE` from the CLI process environment.
- `docker compose config --format json` renders the merged, interpolated, normalized application model without launching containers.
- Container inspection provides the actual image ID. Image inspection supplies a repository digest when available; local builds fall back to the immutable local image ID.

## Exact lifecycle

For each selected path, after static safety and Compose model validation:

```text
from (start full project)
→ wait for HTTP health
→ seed
→ capture seeded evidence
→ via[0] (recreate selected service only)
→ wait and capture
→ ...
→ target (recreate selected service only)
→ wait and capture
→ run every invariant check
→ capture final evidence
→ write JSON/JUnit
→ bounded project-scoped cleanup
```

A local build target is built once per run and reused by all selected paths. Each path still receives a unique Compose project and state.

## Real OSS validation worksheet

For each of 3–5 candidates record repository, stars, topology, persistent services, supported historical path, current upgrade-test approach and orchestration LOC, UpgradeProof integration/seed/verify LOC, elapsed integration time, core special cases, and result. A forked integration is compatibility evidence rather than adoption.

The GO/PIVOT/KILL thresholds remain those in the engineering task: at least three materially different successful integrations, measurable orchestration reduction, understandable project-specific checks, and no repeated product-specific hacks in core.
