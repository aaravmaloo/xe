# xe

`xe` is a Go-style Python toolchain manager with:

- one `xe.toml` per project,
- no virtualenvs,
- a global content-addressed cache (CAS),
- deterministic dependency locking.

## Quick start

```bash
go build -o xe main.go
./xe init
./xe python install 3.12
./xe use 3.12
./xe add requests
./xe run -- python -c "import requests; print(requests.__version__)"
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
