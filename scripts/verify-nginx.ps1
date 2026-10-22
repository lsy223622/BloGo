param(
    [string]$OutputDirectory = 'output/site',
    [string]$Image = 'nginx:stable-alpine'
)
$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$siteRoot = (Resolve-Path -LiteralPath (Join-Path $repositoryRoot $OutputDirectory)).Path
$nginxConfig = Join-Path $repositoryRoot 'deploy/nginx.conf'
$redirectConfig = Join-Path $repositoryRoot 'deploy/redirects.conf'
$containerName = 'blogo-nginx-test-' + [guid]::NewGuid().ToString('N')
$containerID = $null
$handler = [System.Net.Http.HttpClientHandler]::new()
$handler.AllowAutoRedirect = $false
$client = [System.Net.Http.HttpClient]::new($handler)
$client.Timeout = [timespan]::FromSeconds(5)
$checks = [System.Collections.Generic.List[object]]::new()
try {
    $containerID = (& docker run --detach --rm --pull never --name $containerName --label blogo.validation=true --user nginx --cap-drop ALL --security-opt no-new-privileges --read-only --tmpfs /tmp:rw,noexec,nosuid -p '127.0.0.1::8080' --mount "type=bind,source=$siteRoot,target=/srv/blog/current,readonly" --mount "type=bind,source=$nginxConfig,target=/etc/blog/nginx.conf,readonly" --mount "type=bind,source=$redirectConfig,target=/etc/blog/redirects.conf,readonly" --entrypoint nginx $Image -c /etc/blog/nginx.conf -g 'daemon off;').Trim()
    if ($LASTEXITCODE -ne 0 -or $containerID -notmatch '^[a-f0-9]{64}$') { throw 'Isolated Nginx container did not start' }
    $port = (& docker port $containerID '8080/tcp').Trim()
    if ($LASTEXITCODE -ne 0 -or $port -notmatch '^127\.0\.0\.1:(\d+)$') { throw 'Test port is not loopback-only' }
    $baseURL = 'http://' + $port
    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try { $response = $client.GetAsync($baseURL + '/').GetAwaiter().GetResult(); $ready = $response.IsSuccessStatusCode; $response.Dispose() } catch {}
        if ($ready) { break }
        Start-Sleep -Milliseconds 100
    }
    if (-not $ready) { & docker logs $containerID; throw 'Nginx did not become ready' }
    $cases = @(
        @{ Path = '/'; Status = 200 },
        @{ Path = '/posts/welcome/'; Status = 200 },
        @{ Path = '/posts/welcome?x=1'; Status = 301; Location = '/posts/welcome/?x=1' },
        @{ Path = '/posts/welcome/index.html?q=x%20y'; Status = 301; Location = '/posts/welcome/?q=x%20y' },
        @{ Path = '/index.html'; Status = 301; Location = '/' },
        @{ Path = '/not-a-page/'; Status = 404 },
        @{ Path = '/404/'; Status = 404 },
        @{ Path = '/meta/manifest.json'; Status = 404 },
        @{ Path = '/source/config.yaml'; Status = 404 },
        @{ Path = '/stats.db'; Status = 404 },
        @{ Path = '/data/private/report.html'; Status = 404 },
        @{ Path = '/rss.xml'; Status = 200 }
    )
    foreach ($case in $cases) {
        $response = $client.GetAsync($baseURL + $case.Path).GetAwaiter().GetResult()
        try {
            $status = [int]$response.StatusCode
            $location = [string]$response.Headers.Location
            if ($status -ne $case.Status) { throw "$($case.Path): expected $($case.Status), received $status" }
            if ($case.ContainsKey('Location') -and $location -ne $case.Location) { throw "$($case.Path): Location $location" }
            $checks.Add(@{ path = $case.Path; status = $status; location = $location })
        } finally { $response.Dispose() }
    }
    $hashedCSS = Get-ChildItem -LiteralPath (Join-Path $siteRoot 'public/assets') -Filter 'theme.*.css' -File | Select-Object -First 1
    if (-not $hashedCSS) { throw 'No fingerprinted theme stylesheet found' }
    $response = $client.GetAsync($baseURL + '/assets/' + $hashedCSS.Name).GetAwaiter().GetResult()
    try {
        if ([int]$response.StatusCode -ne 200 -or [string]$response.Headers.CacheControl -notmatch 'immutable') { throw 'Hashed asset cache policy is missing' }
        $checks.Add(@{ path = '/assets/' + $hashedCSS.Name; status = 200; cache = [string]$response.Headers.CacheControl })
    } finally { $response.Dispose() }
    $uid = (& docker exec $containerID id -u).Trim()
    if ($uid -eq '0') { throw 'Nginx is running as root' }
    & docker exec $containerID nginx -t -c /etc/blog/nginx.conf
    if ($LASTEXITCODE -ne 0) { throw 'Nginx config validation failed' }
    $version = (& docker exec $containerID nginx -v 2>&1) -join ' '
    $report = @{ nginx = $version; uid = $uid; image = $Image; base_url = $baseURL; checks = $checks }
    $reportRoot = Join-Path $repositoryRoot 'output/validation'
    New-Item -ItemType Directory -Force $reportRoot | Out-Null
    $report | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $reportRoot 'nginx.json')
    Write-Output "Nginx validation passed: $($checks.Count) HTTP/cache checks, uid $uid, $version"
} finally {
    $client.Dispose()
    $handler.Dispose()
    if ($containerID -and $containerID -match '^[a-f0-9]{64}$') {
        $label = (& docker inspect --format '{{index .Config.Labels "blogo.validation"}}' $containerID 2>$null)
        if ($LASTEXITCODE -eq 0 -and $label -eq 'true') { & docker rm --force $containerID | Out-Null }
    }
}
