# Command Reference

This page lists all commands currently exposed by `xe`.

## Global syntax

```bash
xe [command] [flags]
```

Global flag:

- `--config`: custom config file path.

## Top-level commands

| Command | Description |
| :--- | :--- |
| `xe add <package_name>...` | Resolve and install one or more packages into the current project. |
| `xe auth` | Manage authentication tokens used for publishing. |
| `xe build` | Build the current project into a wheel artifact. |
| `xe cache` | Manage the global cache. |
| `xe check <package_name>` | Query package metadata from package index sources. |
| `xe clean` | Remove global and local state managed by xe. |
| `xe completion` | Generate shell completion scripts. |
| `xe doctor` | Check environment health and dependency status. |
| `xe export <output_path>` | Export current cache/environment metadata. |
| `xe format [path]` | Format Python source with `black` through xe runtime. |
| `xe import <path_to_config>` | Import dependencies from a supported config file. |
| `xe init [name]` | Initialize a project and generate `xe.toml`. |
| `xe list` | List dependencies recorded in project config. |
| `xe lock` | Resolve and pin dependency versions in `xe.toml`. |
| `xe mirror` | Manage package index mirror settings. |
| `xe pip` | Package-operation compatibility command group. |
| `xe plugin` | Manage xe plugins. |
| `xe publish` | Publish package (alias behavior for push flow). |
| `xe push` | Upload package to primary package index. |
| `xe python` | Manage Python runtimes and project Python selection. |
| `xe remove <package_name>...` | Remove package entries from project dependency set. |
| `xe restore <name>` | Restore xe state from a named snapshot. |
| `xe run -- [command]` | Run command in project runtime context. |
| `xe self` | Manage xe itself. |
| `xe setup` | Perform one-time setup such as PATH shim wiring. |
| `xe shell` | Open a shell configured for the current project. |
| `xe snapshot <name>` | Create a named snapshot of xe state. |
| `xe sync` | Install dependencies from `xe.toml`. |
| `xe tool` | Tool install/run management commands. |
| `xe tpush` | Upload package to test package index endpoint. |
| `xe tree [package_name]` | Print dependency tree view. |
| `xe use <python_version>` | Install/select project Python version. |
| `xe venv` | Compatibility command; virtualenv management is disabled. |
| `xe version` | Show xe version and platform details. |
| `xe why <package_name>` | Explain dependency inclusion chain. |
| `xe workspace` | Workspace and monorepo helpers. |
| `xe x -- <command>` | Shorthand alias to run tool commands. |

## `xe python`

| Command | Description |
| :--- | :--- |
| `xe python install <version>` | Install a Python runtime version. |
| `xe python list` | List installed runtime directories. |
| `xe python find` | Print executable path for active Python selection. |
| `xe python pin <version>` | Pin project Python version in `xe.toml`. |
| `xe python dir` | Print root path of managed Python installs. |

## `xe pip`

| Command | Description |
| :--- | :--- |
| `xe pip install <pkg>...` | Install packages (alias flow to `xe add`). |
| `xe pip uninstall <pkg>...` | Remove packages (alias flow to `xe remove`). |
| `xe pip list` | List project dependencies. |
| `xe pip show <pkg>` | Show package metadata. |
| `xe pip tree` | Show dependency tree view. |
| `xe pip check` | Run dependency health checks. |
| `xe pip sync` | Sync dependencies from `xe.toml`. |
| `xe pip compile` | Lock dependencies in `xe.toml`. |

## `xe tool`

| Command | Description |
| :--- | :--- |
| `xe tool run -- <command>` | Run a tool command inside xe runtime context. |
| `xe tool install <tool>...` | Add tools to project dependency set. |
| `xe tool list` | List tools tracked in project dependency map. |
| `xe tool update <tool>...` | Update selected tool dependencies. |
| `xe tool uninstall <tool>...` | Remove selected tool dependencies. |
| `xe tool upgrade` | Upgrade all tool dependencies by sync flow. |
| `xe tool sync` | Synchronize tool dependencies with config. |
| `xe tool dir` | Print project tool location. |

## `xe cache`

| Command | Description |
| :--- | :--- |
| `xe cache dir` | Print global cache directory path. |
| `xe cache clean` | Remove all cached artifacts and metadata. |
| `xe cache prune` | Prune stale cache metadata entries. |

## `xe auth`

| Command | Description |
| :--- | :--- |
| `xe auth login` | Store publishing token in secure storage. |
| `xe auth revoke` | Remove stored publishing token. |

## `xe mirror`

| Command | Description |
| :--- | :--- |
| `xe mirror add <url>` | Add a new package index mirror URL. |
| `xe mirror list` | List configured mirror URLs. |

## `xe plugin`

| Command | Description |
| :--- | :--- |
| `xe plugin list` | List installed xe plugins. |

## `xe self`

| Command | Description |
| :--- | :--- |
| `xe self update` | Check/apply xe binary updates. |

## `xe workspace`

| Command | Description |
| :--- | :--- |
| `xe workspace init` | Initialize workspace metadata. |
| `xe workspace add <path>` | Add project path into workspace. |
