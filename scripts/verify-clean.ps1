$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
if ($env:GOOS -and $env:GOOS -ne 'windows') { throw 'Clear cross-compilation GOOS before the Windows clean-checkout test' }
$scratchRoot = Join-Path $repositoryRoot '.superpowers'
New-Item -ItemType Directory -Force $scratchRoot | Out-Null
$checkRoot = Join-Path $scratchRoot ('clean-checkout-' + [guid]::NewGuid().ToString('N'))
$archive = $checkRoot + '.zip'
try {
    & git -C $repositoryRoot archive --format=zip --output=$archive HEAD
    if ($LASTEXITCODE -ne 0) { throw 'Could not archive tracked sources' }
    Expand-Archive -LiteralPath $archive -DestinationPath $checkRoot
    $binary = Join-Path $checkRoot 'blog.exe'
    & go -C $checkRoot build -trimpath -o $binary ./cmd/blog
    if ($LASTEXITCODE -ne 0) { throw 'Clean source compilation failed' }
    & go -C $checkRoot test ./...
    if ($LASTEXITCODE -ne 0) { throw 'Clean source tests failed' }
    $clock = [System.Diagnostics.Stopwatch]::StartNew()
    & $binary build --config (Join-Path $checkRoot 'config.yaml') --output (Join-Path $checkRoot 'output/site')
    if ($LASTEXITCODE -ne 0) { throw 'Clean source site build failed' }
    $clock.Stop()
    $manifest = Get-Content -LiteralPath (Join-Path $checkRoot 'output/site/meta/manifest.json') -Raw | ConvertFrom-Json
    $report = @{ passed = $true; commit = (& git -C $repositoryRoot rev-parse HEAD).Trim(); go = (& go version); pages = $manifest.pages.Count; build_ms = $clock.ElapsedMilliseconds; dependencies = 'go.mod/go.sum; module cache reused' }
    $resultRoot = Join-Path $repositoryRoot 'output/validation'
    New-Item -ItemType Directory -Force $resultRoot | Out-Null
    $report | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $resultRoot 'clean-checkout.json')
    $report | ConvertTo-Json
} finally {
    $expectedParent = [System.IO.Path]::GetFullPath($scratchRoot)
    $resolvedCheck = [System.IO.Path]::GetFullPath($checkRoot)
    if ([System.IO.Path]::GetDirectoryName($resolvedCheck) -ne $expectedParent -or [System.IO.Path]::GetFileName($resolvedCheck) -notlike 'clean-checkout-*') { throw 'Unsafe cleanup target' }
    if (Test-Path -LiteralPath $resolvedCheck) { Remove-Item -LiteralPath $resolvedCheck -Recurse -Force }
    if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
}
