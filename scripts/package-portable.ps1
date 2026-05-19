param(
  [string]$Label = "portable",
  [string]$SourceExe = ""
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$wailsPath = Join-Path $repoRoot "wails.json"
$portableRoot = Join-Path $repoRoot "build\portable"

if (!(Test-Path -LiteralPath $wailsPath)) {
  throw "missing wails.json: $wailsPath"
}

$wails = Get-Content -Raw -LiteralPath $wailsPath | ConvertFrom-Json
$version = [string]$wails.info.productVersion
if ([string]::IsNullOrWhiteSpace($version)) {
  $version = "unknown"
}

if ([string]::IsNullOrWhiteSpace($SourceExe)) {
  $SourceExe = Join-Path $repoRoot "build\bin\res-downloader.exe"
}
if (!(Test-Path -LiteralPath $SourceExe)) {
  throw "missing executable: $SourceExe"
}

$commit = "nogit"
try {
  $commit = (git -C $repoRoot rev-parse --short HEAD).Trim()
} catch {
  $commit = "nogit"
}

$safeLabel = $Label -replace '[^\w.-]+', '-'
$safeLabel = $safeLabel.Trim("-")
if ([string]::IsNullOrWhiteSpace($safeLabel)) {
  $safeLabel = "portable"
}

New-Item -ItemType Directory -Force -Path $portableRoot | Out-Null

$stamp = Get-Date -Format "yyyyMMdd-HHmmss"
$baseName = "res-downloader_$version`_$safeLabel`_$commit`_$stamp"
$packageDir = Join-Path $portableRoot $baseName
$index = 1
while (Test-Path -LiteralPath $packageDir) {
  $packageDir = Join-Path $portableRoot "$baseName-$index"
  $index += 1
}

New-Item -ItemType Directory -Path $packageDir | Out-Null
$targetExe = Join-Path $packageDir "res-downloader.exe"
Copy-Item -LiteralPath $SourceExe -Destination $targetExe

$hash = Get-FileHash -Algorithm SHA256 -LiteralPath $targetExe
$manifest = [ordered]@{
  name = "res-downloader"
  version = $version
  label = $safeLabel
  commit = $commit
  created_at = (Get-Date).ToString("o")
  executable = "res-downloader.exe"
  sha256 = $hash.Hash
}

$manifestPath = Join-Path $packageDir "manifest.json"
$manifest | ConvertTo-Json | Set-Content -LiteralPath $manifestPath -Encoding UTF8

Write-Output $packageDir
