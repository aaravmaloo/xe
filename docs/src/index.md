# xe Documentation

## Overview
`xe` is designed to be the only tool you need for Python development on Windows.

## Core Concepts

### Universal Lockfile
The `xe.lock` file is a deterministic TOML file that ensures reproducible environments.

### Shim Management
`xe` adds a single shim directory to your PATH: `~/.xe/bin/`. All Python versions and package binaries are managed internally via these shims.

## Commands

### `xe use`
Usage: `xe use <version> [--default]`
Installs the specified Python version if not present and sets it as the active version.

### `xe venv`
Usage: `xe venv <name>`
Activates or deactivates a virtual environment.

### `xe snapshot`
Usage: `xe snapshot <name>`
Creates a backup of the entire managed environment.
