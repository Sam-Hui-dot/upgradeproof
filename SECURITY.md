# Security policy

UpgradeProof is experimental. Security fixes target the latest v0.1.x release; reports against unreleased commits or older v0.1.x versions may be asked to reproduce on the latest supported patch.

Please report suspected vulnerabilities through GitHub's private vulnerability reporting for `Sam-Hui-dot/upgradeproof`. Do not include credentials, production data, or other secrets in an issue, fixture, report bundle, or Compose log.

UpgradeProof invokes Docker and repository-owned hook commands with the caller's permissions. Review configuration, Compose files, and hooks as executable project input. Its safety checks reduce accidental interference with host resources but are not a sandbox or a substitute for an isolated CI runner.
