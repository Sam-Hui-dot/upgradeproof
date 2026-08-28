# P0 safety contract

UpgradeProof accepts state only when ownership can be derived from its generated Compose project identity.

Accepted persistent storage is a normal top-level Compose named volume without `external`, without `name`, and without driver options. An omitted driver and explicit `driver: local` are equivalent under this contract when `driver_opts` is empty. Compose prefixes such resources with the project name and labels them with `com.docker.compose.project`. A clearly read-only source/config bind is also accepted.

The preflight rejects:

- `external: true`, because Compose does not own that volume's lifecycle;
- any explicit top-level volume `name`, including interpolated names, because Compose uses it as-is without project scoping;
- writable short- or long-form relative binds;
- writable short- or long-form absolute binds, including Windows absolute/UNC paths;
- bind sources derived from variables when they are writable, because ownership cannot be proven statically;
- local-driver `type: none` plus `o: bind` volume tricks;
- any driver other than `local`, and any non-empty `driver_opts`, including local bind tricks and remote/NFS options, because ownership cannot be proven during the validation phase;
- fixed `container_name` on any service;

For every `from`, `via`, and `to` release environment, the CLI asks Compose for two views before the engine starts the source release. It audits the fully resolved include/extends/merge/interpolation model from `docker compose config --format json --no-normalize`, and independently audits normalized canonical JSON. `--no-normalize` preserves a user-supplied `volume.name`; the canonical view synthesizes `<project>_<volume>` names for safe project-scoped volumes, so only that exact generated pattern is accepted there. The paired, per-state audits prevent both interpolation-dependent bypasses and false positives.

Every path gets a distinct lowercase project name beginning with `upgradeproof-`. The same name is passed via Compose's highest-precedence `-p` option at every lifecycle step. Every release hop runs full-project `compose up -d --remove-orphans`; Compose recreates services whose resolved configuration changed, retains stable services and volumes, and applies its own dependency and one-shot completion semantics. Cleanup is bounded and project-scoped:

```text
docker compose -f <declared-file> -p <generated-project> down --volumes --remove-orphans
```

The cleanup implementation rejects non-UpgradeProof project names. It does not guess Docker resource names and never calls system, volume, or network prune.

For a local build target, `build.tag_env` is replaced with `upgradeproof-target-<run-id>`. Every declared build service must resolve to an image reference ending in that exact tag or preflight fails. After project cleanup, the engine removes only those exact resolved image references. With `--keep-on-failure`, tags are deliberately retained alongside the failed project. BuildKit's internal cache is outside this ownership model and is not pruned.

The semantics were checked against current Docker documentation during the implementation pass:

- https://docs.docker.com/reference/compose-file/volumes/
- https://docs.docker.com/reference/cli/docker/compose/up/
- https://docs.docker.com/reference/cli/docker/compose/config/
- https://docs.docker.com/compose/how-tos/project-name/
- https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/
