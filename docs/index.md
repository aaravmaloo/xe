# xe Documentation

`xe` is a Python toolchain manager written in Rust with a no-virtualenv runtime model.

## What xe provides

- Project initialization with a single `xe.toml`.
- Python runtime install, selection, and pinning.
- Dependency add/remove/list/check with lock and sync workflows.
- Global content-addressed cache for package artifacts and solve metadata.
- Project-local package installation into `.xe/site-packages`.
- Command execution with project package wiring via `PYTHONPATH`.
- Packaging and publishing commands (`build`, `push`, `publish`, `tpush`).
- Authentication token management for package publishing.
- Cache, mirror, plugin, snapshot, workspace, and self-management command groups.
- Compatibility command surfaces under `pip` and `tool`.

## Principles

- One project file: `xe.toml`.
- No virtual environment creation or activation required.
- Shared cache outside the project directory.
- Deterministic lock and sync flow.
- High-throughput parallel resolve/download/install pipeline.

## Documentation map

- [Getting Started](getting-started.md)
- [Configuration](configuration.md)
- [Workflows](workflows.md)
- [Architecture](architecture.md)
- [Command Reference](commands.md)
- [Performance](performance.md)
- [Security](security.md)
- [Troubleshooting](troubleshooting.md)
