$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
Set-Location $repoRoot

cargo build --release --manifest-path rust/xe_cli/Cargo.toml | Out-Host

$builtExe = Join-Path $repoRoot "rust/xe_cli/target/release/xe.exe"
$finalExe = Join-Path $repoRoot "xe.exe"
Copy-Item -Path $builtExe -Destination $finalExe -Force

Write-Host "Built $finalExe"
