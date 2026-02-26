# Security

## Credential storage

Publishing credentials are stored using platform-specific secure handling:

- Windows: Credential Manager integration.
- Linux: restricted file storage in user home.

## Integrity model

- Download artifacts are hash-checked when digest metadata is available.
- Artifacts are stored in content-addressed cache paths.
- Dependency resolution metadata is cached separately from blob storage.

## Operational recommendations

- Use scoped package index tokens with minimal permissions.
- Rotate credentials regularly with `xe auth revoke` and `xe auth login`.
- Keep lockfiles committed for deterministic installs.
- Run `xe doctor` in CI to catch runtime and dependency issues early.

## Cleanup and incident response

- `xe clean` removes local and global xe-managed state.
- `xe cache clean` removes cached package artifacts.
- `xe restore <snapshot>` can return to known-good state if snapshots are used.
