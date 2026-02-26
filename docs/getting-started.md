# Getting Started

## Build xe

```bash
go build -o xe main.go
```

On Windows:

```powershell
go build -o xe.exe main.go
```

## Initial setup

```bash
xe setup
```

This adds the shim directory to PATH so `xe`-managed commands are reachable.

## Create a project

```bash
xe init
```

This creates:

- `xe.toml`
- `.xe/site-packages`

## Choose Python

```bash
xe python install 3.12
xe use 3.12
```

## Add dependencies

```bash
xe add requests flask
```

## Lock and sync

```bash
xe lock
xe sync
```

## Run commands

```bash
xe run -- python -c "import requests; print(requests.__version__)"
xe shell
```

## First project checklist

1. `xe init`
2. `xe use <version>`
3. `xe add <packages>`
4. `xe lock`
5. `xe run -- ...`
