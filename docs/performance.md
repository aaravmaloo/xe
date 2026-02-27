# Performance

`xe` is optimized around high-throughput package management and predictable install behavior.

## Performance pillars

| Pillar | Implementation | Benefit |
| :--- | :--- | :--- |
| Parallel resolution | Concurrent dependency solving per requirement | Faster graph construction |
| Solve cache reuse | Pre-solved graph cache keyed by inputs | Lower repeated resolver cost |
| Content-addressed artifacts | Blobs keyed by digest | Deduplicated storage and cache hit speed |
| Download planning | Planned artifact retrieval before install | Reduced redundant transfer |
| Streamed extraction | Extract wheel content into project target | Lower intermediate filesystem overhead |
| Project-local install target | `.xe/site-packages` with direct extraction | Fast runtime activation |

## Cache model

- Global cache root:
  - Windows: `%LOCALAPPDATA%/xe/cache`
  - Linux/macOS: `~/.cache/xe`
- Blobs are keyed by SHA-256.
- Solve graphs are cached separately from artifact blobs.

## Execution pipeline summary

1. Parse requirements and project config.
2. Attempt solve cache hit.
3. Resolve dependencies in parallel on cache miss.
4. Build download plan and fill cache from network when needed.
5. Install artifacts to project `.xe/site-packages`.
6. Run commands with runtime path wiring.

## Operational guidance

- Run `xe lock` after adding packages to keep resolution deterministic.
- Use `xe sync` in CI to align installs with `xe.toml`.
- Keep cache warm across builds for best throughput.
- Prefer shared cache persistence between CI jobs for lower cold-start time.

## Profiling slow paths

`xe` now supports built-in profiling with structured timing logs:

```bash
xe --profile add requests
xe --profile --profile-dir .xe/profiles sync
```

Each profiled run writes three artifacts:

- `trace-<timestamp>.jsonl`: structured timing events for spans such as runtime setup, resolve, CAS download, and wheel extraction.
- `cpu-<timestamp>.pprof`: CPU profile.
- `heap-<timestamp>.pprof`: heap profile captured at command end.

To inspect the CPU profile:

```bash
go tool pprof -top ./xe /path/to/cpu-*.pprof
```
