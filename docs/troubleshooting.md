# Troubleshooting

## Command not found

Symptom: `xe` or shimmed executables are not recognized.

Fix:

1. Run `xe setup`.
2. Restart terminal.
3. Verify PATH contains xe shim directory.

## Python version not available

Symptom: runtime resolution or command execution fails for selected version.

Fix:

1. Run `xe python install <version>`.
2. Run `xe use <version>`.
3. Confirm with `xe python find`.

## Dependencies not importable

Symptom: `ModuleNotFoundError` for installed package.

Fix:

1. Run `xe sync`.
2. Ensure command is executed with `xe run -- ...` or `xe shell`.
3. Confirm package exists in `.xe/site-packages`.

## Lock/sync mismatch

Symptom: installed environment does not reflect config.

Fix:

1. Run `xe lock`.
2. Run `xe sync`.
3. Re-run with `xe cache clean` if stale artifacts are suspected.

## Publish failures

Symptom: push/publish errors related to auth or upload.

Fix:

1. Run `xe auth revoke`.
2. Run `xe auth login`.
3. Retry `xe build` then `xe publish` or `xe push`.

## Cache corruption suspicion

Symptom: repeated checksum or extraction failures.

Fix:

1. Run `xe cache clean`.
2. Run `xe sync` again to rebuild cache content.

## Last-resort reset

If environment is unrecoverable:

```bash
xe clean
xe init
xe use <version>
xe sync
```
