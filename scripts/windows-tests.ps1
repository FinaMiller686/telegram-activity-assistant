# Run manually with PowerShell. Uses only temporary files and a fake Docker command.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'windows-common.ps1')

function Assert-True { param([bool]$Value, [string]$Message) if (-not $Value) { throw $Message } }
function Assert-Throws {
    param([scriptblock]$Action, [string]$Expected)
    try { & $Action } catch {
        Assert-True ($_.Exception.Message.Contains($Expected)) ('Unexpected error in: ' + $Expected)
        return
    }
    throw ('Expected failure: ' + $Expected)
}

$root = Join-Path ([IO.Path]::GetTempPath()) ('activity-windows-test-' + [Guid]::NewGuid().ToString('N'))
$project = Join-Path $root ('path with spaces ' + [char]0x6D4B)
$null = New-Item -ItemType Directory -Path $project -Force
$script:protectedPaths = @()
$script:failProtection = $false
function Protect-ActivityPath {
    param([string]$Path, [switch]$Directory)
    $script:protectedPaths += $Path
    if ($script:failProtection -and $Path.EndsWith('master.key')) { throw 'Simulated ACL error' }
}
$script:dockerCalls = @()
$script:dockerMode = 'ok'
function docker {
    $script:dockerCalls += ,@($args)
    $global:LASTEXITCODE = 0
    if ($script:dockerMode -eq 'offline') { $global:LASTEXITCODE = 1; return }
    if ($args[0] -eq 'info') {
        if ($script:dockerMode -eq 'windows') { 'windows' } else { 'linux' }
    } elseif ($args[0] -eq 'compose' -and $args[1] -eq 'version') {
        if ($script:dockerMode -eq 'old') { 'v2.1.0' } else { 'v2.40.0' }
    } elseif ($script:dockerMode -eq 'failure') { $global:LASTEXITCODE = 7 }
}

try {
    foreach ($name in @('windows.ps1', 'windows-common.ps1')) {
        $parseErrors = $null
        $tokens = $null
        $null = [Management.Automation.Language.Parser]::ParseFile((Join-Path $PSScriptRoot $name), [ref]$tokens, [ref]$parseErrors)
        Assert-True ($parseErrors.Count -eq 0) ('Parse failed: ' + $name)
    }
    $example = "TGTLDR_DB_PASSWORD=`r`nTGTLDR_HOST_WEB_PORT=3000`r`nTGTLDR_STORAGE_PROTECTED=false`r`n"
    [IO.File]::WriteAllText((Join-Path $project '.env.example'), $example)
    Assert-True ((Get-ActivitySetting $example 'TGTLDR_DB_PASSWORD') -eq '') 'Empty setting consumed the next line'
    Assert-Throws { Get-ActivitySetting "X=1`nX=2" 'X' } 'Duplicate setting'
    Assert-True (Initialize-ActivityWindows $project) 'Fresh setup failed'
    $envPath = Join-Path $project '.env'
    $keyPath = Join-Path $project '.local/windows/master.key'
    $envBefore = [IO.File]::ReadAllText($envPath)
    $keyBefore = [IO.File]::ReadAllText($keyPath)
    Assert-True ((Get-ActivitySetting $envBefore 'TGTLDR_DB_PASSWORD') -match '^[a-f0-9]{64}$') 'Invalid generated database password'
    Assert-True ([Convert]::FromBase64String($keyBefore.Trim()).Length -eq 32) 'Invalid generated key'
    Assert-True ($script:protectedPaths -contains $keyPath) 'Key ACL was not requested'
    Assert-True (@($script:protectedPaths | Where-Object { $_ -like '*.pending' }).Count -gt 0) 'Environment ACL was not requested'
    Assert-True (-not (Initialize-ActivityWindows $project)) 'Repeated setup did not preserve existing settings'
    Assert-True ([IO.File]::ReadAllText($keyPath) -ceq $keyBefore) 'Repeated setup rotated the key'
    Assert-True ([IO.File]::ReadAllText($envPath) -ceq $envBefore) 'Repeated setup changed the environment'
    Assert-Throws { Get-ActivityWindowsConfig $project } 'Confirm encryption'
    $confirmed = Set-ActivitySetting $envBefore 'TGTLDR_STORAGE_PROTECTED' 'true'
    [IO.File]::WriteAllText($envPath, (Set-ActivitySetting $confirmed 'TGTLDR_HOST_WEB_PORT' '13000'))
    Assert-True ((Get-ActivityWindowsConfig $project).Port -eq 13000) 'Custom port was ignored'
    [IO.File]::WriteAllText($envPath, (Set-ActivitySetting $confirmed 'TGTLDR_HOST_WEB_PORT' '99999'))
    Assert-Throws { Get-ActivityWindowsConfig $project } 'between 1 and 65535'
    [IO.File]::WriteAllText($envPath, $confirmed)
    [IO.File]::WriteAllText($keyPath, 'invalid')
    Assert-Throws { Get-ActivityWindowsConfig $project } 'Master key is invalid'
    Assert-Throws { Initialize-ActivityWindows $project } 'Master key is invalid'
    [IO.File]::WriteAllText($keyPath, $keyBefore)
    Remove-Item -LiteralPath $keyPath
    Assert-Throws { Initialize-ActivityWindows $project } 'Master key is missing'
    Assert-True (-not (Test-Path -LiteralPath $keyPath)) 'Missing existing key was regenerated'
    Assert-True ([IO.File]::ReadAllText($envPath) -ceq $confirmed) 'Failure changed existing settings'
    $failedSetup = Join-Path $root 'acl failure'
    $null = New-Item -ItemType Directory -Path $failedSetup
    [IO.File]::WriteAllText((Join-Path $failedSetup '.env.example'), $example)
    $script:failProtection = $true
    Assert-Throws { Initialize-ActivityWindows $failedSetup } 'could not save'
    Assert-True (-not (Test-Path (Join-Path $failedSetup '.env'))) 'ACL failure saved an environment file'
    Assert-True (-not (Test-Path (Join-Path $failedSetup '.local/windows/master.key'))) 'ACL failure left an unprotected key'
    $script:failProtection = $false
    Assert-True (Initialize-ActivityWindows $failedSetup) 'Setup did not recover from a simulated ACL failure'
    Assert-ActivityDocker
    foreach ($mode in @('windows', 'offline', 'old')) {
        $script:dockerMode = $mode
        $expected = switch ($mode) { 'windows' { 'Linux containers' } 'offline' { 'not ready' } 'old' { '2.20' } }
        Assert-Throws { Assert-ActivityDocker } $expected
    }
    $script:dockerMode = 'ok'
    Invoke-ActivityCompose $project @('up', '-d', '--wait')
    $call = $script:dockerCalls[-1]
    Assert-True ($call[2] -ceq $project) 'Path with spaces was split'
    Assert-True ($call -contains (Join-Path $project 'docker-compose.windows.yml')) 'Windows key mount was omitted'
    Assert-True ($call -contains '--wait') 'Readiness check was omitted'
    Assert-True (-not ($call -contains '-v')) 'Destructive volume flag was included'
    $script:dockerMode = 'failure'
    Assert-Throws { Invoke-ActivityCompose $project @('up', '-d') } 'Docker Compose failed'
    Write-Host 'Windows script tests passed (temporary files, mocked ACL and Docker; no Windows host or containers tested).'
} finally {
    if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force }
}
