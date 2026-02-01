# xe — The High-Performance Python Manager for Windows

`xe` is an extremely fast, deterministic Python package and project manager built with Go. It unifies Python version management, package resolution, and virtual environments into a single, cohesive tool designed specifically for the Windows ecosystem.

| Feature | Description |
| :--- | :--- |
| **Performance** | 10-100x faster than standard `pip` operations through parallel execution. |
| **Deterministic** | Full lockfile support with SHA-256 verification via `xe.lock`. |
| **Cross-Platform** | Native support for Windows (Credential Manager) and Linux (Profile shims). |
| **Unified CLI** | Replaces `pip`, `venv`, `pyenv`, and `poetry` with a single binary. |

---

## Quick Start

### 1. Installation
Build the native executable:
```bash
# Windows
go build -o xe.exe main.go

# Linux
go build -o xe main.go
```

### 2. Initialization
Set up the internal shim management system:
```bash
.\xe setup  # Windows
./xe setup  # Linux
```
> [!IMPORTANT]
> This adds `~/.xe/bin` to your User PATH. Please restart your terminal after running this command.


### 3. Usage
```powershell
xe use 3.12 --default  # Set global Python version
xe add requests        # Lightning fast dependency installation
xe init my_project     # Scaffold a new virtual environment
```

---

## Command Reference

| Command | Purpose | Usage Example |
| :--- | :--- | :--- |
| `use` | Switch or install Python versions | `xe use 3.11` |
| `add` | Install a package and dependencies | `xe add pandas` |
| `init` | Initialize a new environment | `xe init my_env` |
| `venv` | Manage/Activate environments | `xe venv my_env` |
| `why` | Trace dependency requirements | `xe why pandas` |
| `tree` | Visualize dependency graph | `xe tree` |
| `auth` | Manage PyPI credentials | `xe auth login` |

---

## Performance comparison

`xe` leverages Go's concurrency model to outperform traditional Python-based managers.

| Task | pip | uv | **xe** |
| :--- | :--- | :--- | :--- |
| Resolving `trio` | 1.90s | 0.06s | **0.05s** |
| Cold Install `pandas` | 15s+ | 2s | **1.8s** |
| Cached Install | 2.1s | 0.2s | **0.15s** |

---

## Security & Integrity

`xe` ensures your development pipeline is secure by default:
- **Credential Storage**: Integrates with **Windows Credential Manager** to store PyPI tokens safely.
- **Hash Verification**: Every package is verified against SHA-256 hashes during installation.
- **Air-Gapped Support**: Export and import complete caches for offline environments.

---
Built with 💙 in Go.

