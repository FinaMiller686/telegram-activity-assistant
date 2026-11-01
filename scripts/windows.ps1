#Requires -Version 5.1
[CmdletBinding()]
param(
    [Parameter(Position = 0)]
    [ValidateSet('setup', 'start', 'stop', 'status', 'logs')]
    [string]$Action = 'start',
    [switch]$NoBrowser
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-common.ps1')

try {
    if ([Environment]::OSVersion.Platform -ne [PlatformID]::Win32NT) {
        throw 'This launcher is for Windows. Use scripts/macos-native.sh on macOS.'
    }
    $projectRoot = Split-Path -Parent $PSScriptRoot
    if ($Action -eq 'setup') {
        $created = Initialize-ActivityWindows -ProjectRoot $projectRoot
        if ($created) { Write-Host 'Created local database password and master key.' }
        else { Write-Host 'Existing settings and master key kept.' }
        Write-Host 'Next: confirm host disk encryption as described in docs/WINDOWS.md, then run start.'
        exit 0
    }
    Assert-ActivityDocker
    switch ($Action) {
        'start' {
            $configuration = Get-ActivityWindowsConfig -ProjectRoot $projectRoot
            Invoke-ActivityCompose -ProjectRoot $projectRoot -ComposeArguments @('config', '--quiet')
            Invoke-ActivityCompose -ProjectRoot $projectRoot -ComposeArguments @('up', '-d', '--build', '--wait', '--wait-timeout', '180')
            $address = 'http://localhost:' + $configuration.Port
            Write-Host ('Started: ' + $address)
            if (-not $NoBrowser) { Start-Process $address }
        }
        'stop' { Invoke-ActivityCompose -ProjectRoot $projectRoot -ComposeArguments @('stop') }
        'status' { Invoke-ActivityCompose -ProjectRoot $projectRoot -ComposeArguments @('ps') }
        'logs' { Invoke-ActivityCompose -ProjectRoot $projectRoot -ComposeArguments @('logs', '--tail', '100', 'app', 'web') }
    }
} catch {
    Write-Host $_.Exception.Message -ForegroundColor Red
    exit 1
}
