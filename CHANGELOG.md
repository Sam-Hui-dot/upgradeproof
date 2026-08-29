# Changelog

All notable changes to UpgradeProof will be documented here. The project follows Semantic Versioning for released CLI behavior and freezes configuration schema `version: 2` for the v0.1.x line.

## [Unreleased]

## [v0.1.0] - 2026-08-29

- Compose release-state upgrade paths with ordered `from`, optional `via`, and `to` environments.
- Optional multi-service local target builds using exact run-owned image tags.
- Raw and fully resolved Compose safety audits for every release state.
- Persistent-state seed and verification hooks with JSON and JUnit evidence.
- GitHub Action installation with SHA256 verification.
- Cross-compiled archives for Linux, macOS, and Windows.
