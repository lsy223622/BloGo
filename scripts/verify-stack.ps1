param([string]$Image = 'nginx:stable-alpine')
$ErrorActionPreference = 'Stop'
$repositoryRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).Path
$results = Join-Path $repositoryRoot 'output/validation'
New-Item -ItemType Directory -Force $results | Out-Null
$containerName = 'blogo-stack-test-' + [guid]::NewGuid().ToString('N')
$containerID = $null
try {
    $containerID = (& docker create --pull never --name $containerName --label blogo.validation=true --network none --read-only --tmpfs /tmp:rw,exec,nosuid --mount "type=bind,source=$repositoryRoot,target=/repo,readonly" --mount "type=bind,source=$results,target=/results" --workdir /repo/internal/integration --env BLOGO_DEPLOY_TEST=1 --env BLOGO_TEST_RESULT=/results/stack.json --entrypoint /repo/.superpowers/linux/integration.test $Image '-test.v' '-test.run' '^TestDeployedLinux$').Trim()
    if ($LASTEXITCODE -ne 0 -or $containerID -notmatch '^[a-f0-9]{64}$') { throw 'Could not create isolated stack test' }
    & docker start --attach $containerID
    $code = (& docker inspect --format '{{.State.ExitCode}}' $containerID).Trim()
    if ($LASTEXITCODE -ne 0 -or $code -ne '0') { throw "Stack verification failed ($code)" }
} finally {
    if ($containerID -and $containerID -match '^[a-f0-9]{64}$') {
        $label = (& docker inspect --format '{{index .Config.Labels "blogo.validation"}}' $containerID 2>$null)
        if ($LASTEXITCODE -eq 0 -and $label -eq 'true') { & docker rm --force $containerID | Out-Null }
    }
}
