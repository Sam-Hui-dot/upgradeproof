# Contributing

UpgradeProof is in an experimental release-candidate phase. Please open an issue before proposing a new abstraction. Focused correctness, safety, documentation, and portability fixes are welcome; project-specific orchestration and new lifecycle DSLs are intentionally out of scope.

## Development checks

Use Go 1.23, Docker, and Docker Compose v2. Before submitting a change, run:

```sh
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
go test -race ./...
```

The Linux CI workflow additionally runs the resolved Compose safety regressions and Fixtures A, B, and C. Fixture B is intentionally broken: its harness succeeds only when UpgradeProof reports the failed upgrade invariant.

Do not commit generated evidence, release archives, locally built binaries, credentials, or upstream project source. By contributing, you agree that your contribution is licensed under Apache-2.0.
