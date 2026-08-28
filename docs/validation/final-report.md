# Real OSS Validation Phase report

## Decision

Final recommendation: **PIVOT**.

The current abstraction is validated for its narrow target class: one versioned application service with run-owned persistent state, optionally alongside a stable database service. It is not validated as a general Compose release-upgrade abstraction.

The leading product pivot is evidence-backed coordinated release/service groups. Spliit Cloud proved that one release may require an ordered one-shot migration plus synchronized API, worker, and web image replacement. That concept was recorded but not implemented in this phase.

The leading adoption/safety question is a separate external-state contract. Labby and Notifuse showed that writable host binds and explicit volume-driver declarations are common enough to block zero-change integrations. Safety was not weakened, and no broad exception is recommended without a provable ownership and cleanup model.

## Compatibility validated

| Repository | Stars snapshot | Tested path | Result without core change | Classification / boundary |
| --- | ---: | --- | --- | --- |
| `truenormis/savvy` | 44 | `v1.2.2 -> v1.2.3` | passed | Named-volume SQLite, single service |
| `joshuamyers-dev/sleeplab` | 21 | `v1.3.1 -> v1.4.0` | passed | App replacement over persistent PostgreSQL |
| `antonio-ivanovski/spliit-cloud` | 51 | `sha-5c9abdc6 -> sha-f1a825489` | failed as expected | **Product limitation:** coordinated migrate/API/worker/web release not expressible |
| `samuelloranger/labby` | 87 | `v1.3.0 -> v1.4.0` | dedicated topology passed; production topology rejected | **Safety limitation:** writable production host bind |
| `Notifuse/notifuse` | 2,085 | `v37.2 -> v38.0` | dedicated topology passed; repository Compose rejected | **Safety limitation:** writable bind and explicit `driver: local` |

Compatibility conclusions:

- Three real published release paths passed without any UpgradeProof core change: Savvy, SleepLab, and the dedicated Notifuse topology.
- Labby also passed under a clearly disclosed named-volume compatibility topology, but its documented production bind topology was not validated.
- Spliit Cloud remained intentionally red and correctly demonstrated the core's single-versioned-service boundary.
- Every pass used real persisted application state and target migration assertions, not merely healthy containers.
- No project business code was changed. All prototype files are local validation-only Compose/config/seed/verify glue.

## External adoption

**External adoption: 0 confirmed repositories.**

No upstream received a pull request. No maintainer accepted, merged, endorsed, or deployed UpgradeProof. Local clones and compatibility prototypes must not be described as adoption.

## Evidence index

| Repository | Structured result | Canonical evidence |
| --- | --- | --- |
| Savvy | `docs/validation/savvy.md` | `D:\Projects\UpgradeProof-validation\evidence\savvy\20260828t142631-cb3310b0f801` |
| SleepLab | `docs/validation/sleeplab.md` | `D:\Projects\UpgradeProof-validation\evidence\sleeplab\20260828t143023-73fc0db70c26` |
| Spliit Cloud | `docs/validation/spliit-cloud.md` | `D:\Projects\UpgradeProof-validation\evidence\spliit-cloud\20260828t143901-78f947b77dd0` |
| Labby | `docs/validation/labby.md` | `D:\Projects\UpgradeProof-validation\evidence\labby\20260828t144339-b73852eea00f` |
| Notifuse | `docs/validation/notifuse.md` | `D:\Projects\UpgradeProof-validation\evidence\notifuse\20260828t145651-d26a0177e700` |

The first-three checkpoint is recorded in `docs/validation/interim-decision.md` as **PIVOT, then continue evidence collection**. The last two projects reinforced the pivot: preserve the narrow validated core, investigate coordinated releases as a product concept, and separately decide whether a safe non-owned/external-state mode is acceptable.

## Scope controls

- UpgradeProof core changes: none
- Project business-code changes: none
- Upstream pull requests: none
- Git tags or GitHub Releases created: none
- Package publication: none
- External OSS integration/adoption claim: none
- Prohibited feature work (`dashboard`, AI, `doctor`, `init`): none

Validation evidence collection stops here for review.
