param([string]$SshHost = "bt")
Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
if ($SshHost -notmatch '^[A-Za-z0-9][A-Za-z0-9_.@-]*$') { throw "Invalid SSH host" }
$command = 'docker exec -i yueli-pg-core sh -c ''exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d identity'''
Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot "register-client.sql") |
    & ssh -o BatchMode=yes $SshHost $command
if ($LASTEXITCODE -ne 0) { throw "Online test client registration failed; no conflicting client was overwritten." }
