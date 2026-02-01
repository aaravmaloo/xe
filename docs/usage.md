# Usage Guide

This guide provides deep dives into the most common `xe` workflows.

## Python Version Management

`xe` manages Python runtimes independently of your system installation.

| Command | Action |
| :--- | :--- |
| `xe use 3.12` | Installs Python 3.12 if not present and sets it for the current project. |
| `xe use 3.11 -d` | Sets 3.11 as the global default (updates your `python` shim). |
| `xe run 3.10 app.py` | Runs a script with a specific version without switching. |

## Package Management

Package operations are parallelized by default.

### Adding Packages
```powershell
xe add numpy pandas matplotlib
```
This command:
1. Resolves all dependencies concurrently.
2. Checks the global cache for existing wheels.
3. Downloads missing packages in parallel.
4. Extracts them directly to the active environment.

### Introspection
| Command | Result |
| :--- | :--- |
| `xe why pandas` | Shows the dependency chain that required pandas. |
| `xe tree` | Prints a beautiful, colorized tree of your environment. |
| `xe list` | Tabular view of all installed packages and their versions. |

## Virtual Environments

`xe` environments are lightweight and deterministic.

```powershell
xe init .       # Initialize in current directory
xe venv activate # Activate for current shell session
```

> [!TIP]
> Use `xe.toml` to share environment configurations with your team. It contains the locked Python version and environment name.
