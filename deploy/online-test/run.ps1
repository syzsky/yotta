# This process and its children are owned by the Workspace Environment session.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$app = $null
try {
    [Environment]::SetEnvironmentVariable('YOTTA_REGISTRY_AUDIENCE', ' ', 'Process')
    [Environment]::SetEnvironmentVariable('FRONTEND_DEVSERVER_URL', $null, 'Process')
    if (-not (Test-Path -LiteralPath $env:ONLINE_YOTTA_EXE -PathType Leaf)) { throw 'ONLINE_YOTTA_EXE does not exist' }
    $ready = $false
    for ($attempt = 0; $attempt -lt 30; $attempt++) {
        try {
            foreach ($url in @($env:YOTTA_REGISTRY_URL, $env:YOTTA_HUB_URL)) {
                Invoke-WebRequest -Uri ($url.TrimEnd('/') + '/healthz') -TimeoutSec 3 -UseBasicParsing | Out-Null
            }
            $ready = $true
            break
        } catch { Start-Sleep -Milliseconds 500 }
    }
    if (-not $ready) { throw 'Online Registry / Hub health checks failed' }
    $app = Start-Process -FilePath $env:ONLINE_YOTTA_EXE -WorkingDirectory (Split-Path $env:ONLINE_YOTTA_EXE) -WindowStyle Normal -PassThru
    while (-not $app.WaitForExit(1000)) {
    }
    if ($app.ExitCode -ne 0) { throw "Yotta exited with code $($app.ExitCode)" }
} finally {
    if ($app -and -not $app.HasExited) { Stop-Process -Id $app.Id -ErrorAction SilentlyContinue }
}
