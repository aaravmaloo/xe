# Configuration

## Project file: `xe.toml`

`xe.toml` is the authoritative project configuration.

Example:

```toml
[project]
name = "my-project"

[python]
version = "3.12"

[deps]
requests = "2.32.5"
flask = "3.1.2"

[cache]
mode = "global-cas"
global_dir = "C:\\Users\\you\\AppData\\Local\\xe\\cache"
```

## Sections

### `[project]`

- `name`: display/project name.

### `[python]`

- `version`: selected Python version for this project.

### `[deps]`

- map of package name to version.
- `"*"` means unconstrained; `xe lock` replaces with resolved versions.

### `[cache]`

- `mode`: cache mode (`global-cas`).
- `global_dir`: absolute path to shared cache storage.

## Global config

Global defaults are read from:

- `~/.xe/config.yaml`

Key currently used:

- `default_python`: fallback Python version when a project file is absent.

## Runtime path model

- Project packages: `./.xe/site-packages`
- Shared cache:
  - Windows: `%LOCALAPPDATA%/xe/cache`
  - Linux/macOS: `~/.cache/xe`
- Python installs:
  - Windows: `%USERPROFILE%/AppData/Local/Programs/Python`
  - Linux/macOS: `~/.xe/python`
