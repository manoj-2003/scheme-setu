# Start the Scheme Setu API against the committed demo cache: no SerpApi key,
# no network calls, no credits spent. The PowerShell counterpart of
# `make offline`, which needs sh-style environment prefixes and is therefore
# unusable on a stock Windows machine.
#
#   .\run-offline.ps1
#
# Then, in a second terminal:  cd web; npm install; npm run dev

$ErrorActionPreference = 'Stop'

Set-Location -Path $PSScriptRoot

$cache = 'fixtures/serp-cache.sqlite'
if (-not (Test-Path $cache)) {
    throw "Missing $cache. Warm it with 'go run ./cmd/warm' (spends SerpApi credits)."
}

$env:SERP_MODE = 'cache'
$env:SERP_CACHE_PATH = $cache

Write-Host "Serving from $cache in cache-only mode: zero SerpApi credits." -ForegroundColor Green
go run ./cmd/server
