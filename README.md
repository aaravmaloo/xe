# xe

`xe` is a Rust-first Python toolchain manager with:

- one `xe.toml` per project,
- no virtualenvs,
- a global content-addressed cache (CAS),
- deterministic dependency locking.

## Quick start

```powershell
cargo build --release --manifest-path rust/xe_cli/Cargo.toml
.\rust\xe_cli\target\release\xe.exe init
.\rust\xe_cli\target\release\xe.exe python install 3.12
.\rust\xe_cli\target\release\xe.exe use 3.12
.\rust\xe_cli\target\release\xe.exe add requests
.\rust\xe_cli\target\release\xe.exe run -- python -c "import requests; print(requests.__version__)"
```

## Core workflow

- `xe init`: create `xe.toml` and `.xe/site-packages`.
- `xe add <pkg>`: resolve + cache + install artifacts.
- `xe lock`: pin dependencies in `xe.toml`.
- `xe sync`: install from `xe.toml`.
- `xe run -- ...`: run commands with project packages on `PYTHONPATH`.

## Command surface

- `xe sync`, `xe lock`, `xe export`, `xe tree`, `xe format`
- `xe python install|list|find|pin|dir`
- `xe pip install|uninstall|list|show|tree|check|sync|compile`
- `xe tool run|install|list|update|uninstall|upgrade|sync|dir`
- `xe cache dir|clean|prune`
- `xe publish`, `xe auth`, `xe build`

## Documentation

Project docs are in MkDocs format.

```bash
mkdocs serve
mkdocs build
```

## Profiling

Use `--profile` on any command to capture timing logs and pprof artifacts:

```bash
./xe --profile add requests
```

## Build Single EXE (Rust Only)

Windows build steps (single `xe.exe`):

```powershell
.\scripts\build_windows_rust.ps1
```

Manual equivalent:

```powershell
cargo build --release --manifest-path rust/xe_cli/Cargo.toml
Copy-Item .\rust\xe_cli\target\release\xe.exe .\xe.exe -Force
```
