# Workflows

## Standard dependency workflow

```bash
xe init
xe use 3.12
xe add requests flask
xe lock
xe sync
xe run -- python app.py
```

## Existing repository onboarding

```bash
xe init
xe import xe.toml
xe sync
```

## Package removal workflow

```bash
xe remove requests
xe lock
xe sync
```

Remove everything:

```bash
xe remove all
```

## Tooling workflow

```bash
xe tool install black ruff pytest
xe tool list
xe tool run -- python -m pytest
xe format .
```

## Publishing workflow

```bash
xe auth login
xe build
xe publish
```

Test index flow:

```bash
xe tpush
```

## Cache maintenance workflow

```bash
xe cache dir
xe cache prune
xe cache clean
```

## Python runtime workflow

```bash
xe python install 3.11
xe python list
xe python find
xe python pin 3.11
```

## Snapshot workflow

```bash
xe snapshot before-upgrade
xe restore before-upgrade
```

## Workspace workflow

```bash
xe workspace init
xe workspace add ./services/api
xe workspace add ./services/web
```
