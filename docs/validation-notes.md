# Validation and release-candidate notes

## Current decision state

Real OSS compatibility validation concluded **GO** for the Compose release-state abstraction. Configuration schema `version: 2` is now the public v0.1.x schema and is documented in [configuration.md](configuration.md). The project remains experimental, and no `v0.1.0` tag, GitHub Release, or package publication exists yet.

**External adoption: 0 confirmed repositories.** Compatibility validation is not upstream adoption.

## Compose semantics used by the engine

- `docker compose -p` supplies a unique UpgradeProof-owned project name.
- Every `from`, `via`, and `to` environment resolves and reapplies the complete Compose model.
- Compose decides dependency order and recreates services whose resolved image or configuration changed while retaining stable services and named volumes.
- `service_completed_successfully` and one-shot migration behavior remain Compose semantics; UpgradeProof does not synthesize a dependency graph.
- `docker compose config --format json` provides the merged and interpolated model audited before any project starts.
- Container and image inspection record every materialized service's requested image and actual digest or image ID.

## Exact lifecycle

For each selected path, after every selected path and release state has passed raw, resolved, and canonical safety checks:

```text
from (converge complete project)
→ wait for HTTP health
→ seed persistent state
→ capture evidence
→ via[0] (converge complete project under next environment)
→ wait and capture
→ ...
→ to (optionally build declared target services, then converge complete project)
→ wait and capture
→ run every invariant check
→ capture final evidence and write JSON/JUnit
→ bounded project-scoped cleanup
→ remove each distinct exact run-owned target image tag
```

Each path receives a unique Compose project and persistent state. Local target builds are path/run-owned and are not shared across paths.

## Compatibility validation record

The five project reports retain repository, stars at validation time, tested path, topology, persistent resources, migration mechanism, existing tests, integration LOC/effort, limitations, and result under [`validation/`](validation/). The prototypes are compatibility evidence only and were not submitted upstream.
