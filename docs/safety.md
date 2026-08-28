# P0 safety contract

UpgradeProof accepts state only when ownership can be derived from its generated Compose project identity.

Accepted persistent storage is a normal top-level Compose named volume without `external`, without `name`, and without local-driver bind options. Compose prefixes such resources with the project name and labels them with `com.docker.compose.project`. A clearly read-only source/config bind is also accepted.

The preflight rejects:

- `external: true`, because Compose does not own that volume's lifecycle;
- any explicit top-level volume `name`, including interpolated names, because Compose uses it as-is without project scoping;
- writable short- or long-form relative binds;
- writable short- or long-form absolute binds, including Windows absolute/UNC paths;
- bind sources derived from variables when they are writable, because ownership cannot be proven statically;
- local-driver `type: none` plus `o: bind` volume tricks;
- any custom volume driver or non-empty `driver_opts`, including remote/NFS options, because ownership cannot be proven during the validation phase;
- fixed `container_name` on any service;
- a selected application service that does not interpolate `UPGRADEPROOF_IMAGE`.

The CLI asks Compose for two views before the engine starts the source release. It audits the fully resolved include/extends/merge/interpolation model from `docker compose config --format json --no-normalize`, and then independently audits the standard normalized canonical JSON. `--no-normalize` preserves a user-supplied `volume.name`; the canonical view synthesizes `<project>_<volume>` names for safe project-scoped volumes, so only that exact generated pattern is accepted there. The paired audits prevent both bypasses and false positives.

Every path gets a distinct lowercase project name beginning with `upgradeproof-`. The same name is passed via Compose's highest-precedence `-p` option at every lifecycle step. Upgrade hops recreate only the selected application service with `--no-deps --force-recreate`; named volumes and other services remain in the project. Cleanup is bounded and project-scoped:

```text
docker compose -f <declared-file> -p <generated-project> down --volumes --remove-orphans
```

The cleanup implementation rejects non-UpgradeProof project names. It does not guess Docker resource names and never calls system, volume, or network prune.

For a local build target, the engine creates exactly `upgradeproof-target:<run-id>`. After all path projects have been cleaned up it removes only that exact tag. With `--keep-on-failure`, the tag is deliberately retained alongside the failed project. BuildKit's internal cache is outside this ownership model and is not pruned.

The semantics were checked against current Docker documentation during the implementation pass:

- https://docs.docker.com/reference/compose-file/volumes/
- https://docs.docker.com/reference/cli/docker/compose/up/
- https://docs.docker.com/reference/cli/docker/compose/config/
- https://docs.docker.com/compose/how-tos/project-name/
- https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/
