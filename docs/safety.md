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
- fixed `container_name` on any service;
- a selected application service that does not interpolate `UPGRADEPROOF_IMAGE`.

The CLI also asks Compose to parse, interpolate, normalize, and validate the model with `docker compose config --format json` before the engine starts the source release.

Every path gets a distinct lowercase project name beginning with `upgradeproof-`. The same name is passed via Compose's highest-precedence `-p` option at every lifecycle step. Upgrade hops recreate only the selected application service with `--no-deps --force-recreate`; named volumes and other services remain in the project. Cleanup is bounded and project-scoped:

```text
docker compose -f <declared-file> -p <generated-project> down --volumes --remove-orphans
```

The cleanup implementation rejects non-UpgradeProof project names. It does not guess Docker resource names and never calls system, volume, or network prune.

The semantics were checked against current Docker documentation during the implementation pass:

- https://docs.docker.com/reference/compose-file/volumes/
- https://docs.docker.com/reference/cli/docker/compose/up/
- https://docs.docker.com/reference/cli/docker/compose/config/
- https://docs.docker.com/compose/how-tos/project-name/
- https://docs.docker.com/compose/how-tos/environment-variables/variable-interpolation/
