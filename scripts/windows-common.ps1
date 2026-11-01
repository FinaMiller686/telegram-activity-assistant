Set-StrictMode -Version Latest

function New-ActivityRandomBytes {
    param([int]$Count = 32)
    $bytes = New-Object byte[] $Count
    $generator = [Security.Cryptography.RandomNumberGenerator]::Create()
    try { $generator.GetBytes($bytes) } finally { $generator.Dispose() }
    return ,$bytes
}

function Get-ActivitySetting {
    param([string]$Content, [string]$Name)
    $pattern = '(?m)^[ \t]*' + [regex]::Escape($Name) + '[ \t]*=[ \t]*([^\r\n]*)'
    $found = [regex]::Matches($Content, $pattern)
    if ($found.Count -gt 1) { throw "Duplicate setting: $Name. Remove the duplicate line from .env." }
    if ($found.Count -eq 0) { return '' }
    return $found[0].Groups[1].Value.Trim().Trim('"').Trim("'")
}

function Set-ActivitySetting {
    param([string]$Content, [string]$Name, [string]$Value)
    $null = Get-ActivitySetting -Content $Content -Name $Name
    $pattern = '(?m)^[ \t]*' + [regex]::Escape($Name) + '[ \t]*=[^\r\n]*'
    $line = $Name + '=' + $Value
    if ([regex]::IsMatch($Content, $pattern)) {
        return [regex]::Replace($Content, $pattern, [Text.RegularExpressions.MatchEvaluator]{ param($m) $line })
    }
    return $Content.TrimEnd() + "`n" + $line + "`n"
}

function Protect-ActivityPath {
    param([string]$Path, [switch]$Directory)
    # This function is only called on Windows. Tests replace it without changing real ACLs.
    $user = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $system = New-Object Security.Principal.SecurityIdentifier 'S-1-5-18'
    if ($Directory) {
        $acl = New-Object Security.AccessControl.DirectorySecurity
        $inheritance = [Security.AccessControl.InheritanceFlags]'ContainerInherit,ObjectInherit'
    } else {
        $acl = New-Object Security.AccessControl.FileSecurity
        $inheritance = [Security.AccessControl.InheritanceFlags]::None
    }
    $acl.SetOwner($user)
    $acl.SetAccessRuleProtection($true, $false)
    foreach ($identity in @($user, $system)) {
        $rule = New-Object Security.AccessControl.FileSystemAccessRule($identity, 'FullControl', $inheritance, 'None', 'Allow')
        $acl.AddAccessRule($rule)
    }
    Set-Acl -LiteralPath $Path -AclObject $acl -ErrorAction Stop
}

function Assert-ActivityKey {
    param([string]$Path)
    if (-not [IO.File]::Exists($Path)) { throw 'Master key is missing. Restore the original key; do not generate a replacement for an existing database.' }
    try { $bytes = [Convert]::FromBase64String([IO.File]::ReadAllText($Path).Trim()) }
    catch { throw 'Master key is invalid. Restore the original key.' }
    if ($bytes.Length -ne 32) { throw 'Master key is invalid. Restore the original key.' }
}

function Initialize-ActivityWindows {
    param([string]$ProjectRoot)
    $envPath = Join-Path $ProjectRoot '.env'
    $privatePath = Join-Path $ProjectRoot '.local/windows'
    $keyPath = Join-Path $privatePath 'master.key'
    $content = if ([IO.File]::Exists($envPath)) { [IO.File]::ReadAllText($envPath) } else { [IO.File]::ReadAllText((Join-Path $ProjectRoot '.env.example')) }
    $password = Get-ActivitySetting -Content $content -Name 'TGTLDR_DB_PASSWORD'
    if ($password) {
        Assert-ActivityKey -Path $keyPath
        Protect-ActivityPath -Path $privatePath -Directory
        Protect-ActivityPath -Path $keyPath
        Protect-ActivityPath -Path $envPath
        return $false
    }
    if ([IO.File]::Exists($keyPath)) { throw 'An existing key was found without its database password. Restore .env before continuing.' }
    $null = New-Item -ItemType Directory -Path $privatePath -Force
    Protect-ActivityPath -Path $privatePath -Directory
    $password = ([BitConverter]::ToString((New-ActivityRandomBytes))).Replace('-', '').ToLowerInvariant()
    $content = Set-ActivitySetting -Content $content -Name 'TGTLDR_DB_PASSWORD' -Value $password
    if (-not (Get-ActivitySetting -Content $content -Name 'TGTLDR_HOST_WEB_PORT')) {
        $content = Set-ActivitySetting -Content $content -Name 'TGTLDR_HOST_WEB_PORT' -Value '3000'
    }
    # Setup never asserts that a host disk is encrypted.
    $content = Set-ActivitySetting -Content $content -Name 'TGTLDR_STORAGE_PROTECTED' -Value 'false'
    $pending = Join-Path $privatePath ('env.' + [Guid]::NewGuid().ToString('N') + '.pending')
    $encoding = New-Object Text.UTF8Encoding $false
    $keyCreated = $false
    try {
        $stream = [IO.File]::Open($keyPath, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::None)
        $keyCreated = $true
        try {
            $keyData = $encoding.GetBytes([Convert]::ToBase64String((New-ActivityRandomBytes)) + "`n")
            $stream.Write($keyData, 0, $keyData.Length)
        } finally { $stream.Dispose() }
        Protect-ActivityPath -Path $keyPath
        [IO.File]::WriteAllText($pending, $content.Replace("`r`n", "`n"), $encoding)
        Protect-ActivityPath -Path $pending
        Move-Item -LiteralPath $pending -Destination $envPath -Force
    } catch {
        if ($keyCreated -and [IO.File]::Exists($keyPath)) { Remove-Item -LiteralPath $keyPath }
        if ([IO.File]::Exists($pending)) { Remove-Item -LiteralPath $pending }
        throw 'Setup could not save its private files. Check folder permissions and try again.'
    }
    return $true
}

function Get-ActivityWindowsConfig {
    param([string]$ProjectRoot)
    $path = Join-Path $ProjectRoot '.env'
    if (-not [IO.File]::Exists($path)) { throw 'Run .\scripts\windows.ps1 setup first.' }
    $content = [IO.File]::ReadAllText($path)
    if (-not (Get-ActivitySetting -Content $content -Name 'TGTLDR_DB_PASSWORD')) { throw 'Database password is missing. Restore .env before starting.' }
    Assert-ActivityKey -Path (Join-Path $ProjectRoot '.local/windows/master.key')
    if ((Get-ActivitySetting -Content $content -Name 'TGTLDR_STORAGE_PROTECTED') -cne 'true') {
        throw 'Confirm encryption for the project folder and Docker Desktop data disk, then set TGTLDR_STORAGE_PROTECTED=true in .env. See docs/WINDOWS.md.'
    }
    $portText = Get-ActivitySetting -Content $content -Name 'TGTLDR_HOST_WEB_PORT'
    if (-not $portText) { $portText = '3000' }
    $port = 0
    if (-not [int]::TryParse($portText, [ref]$port) -or $port -lt 1 -or $port -gt 65535) { throw 'TGTLDR_HOST_WEB_PORT must be between 1 and 65535.' }
    return [pscustomobject]@{ Port = $port }
}

function Assert-ActivityDocker {
    if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { throw 'Install and start Docker Desktop with the WSL 2 backend first.' }
    $engine = (& docker info --format '{{.OSType}}' 2>$null | Out-String).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Docker Desktop is not ready. Start it and wait for the engine to finish loading.' }
    if ($engine -ne 'linux') { throw 'Switch Docker Desktop to Linux containers (WSL 2). Windows containers are not supported.' }
    $version = (& docker compose version --short 2>$null | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $version -notmatch '(\d+\.\d+\.\d+)' -or [version]$Matches[1] -lt [version]'2.20.0') {
        throw 'Update Docker Desktop. Docker Compose 2.20 or later is required.'
    }
}

function Invoke-ActivityCompose {
    param([string]$ProjectRoot, [string[]]$ComposeArguments)
    $arguments = @('compose', '--project-directory', $ProjectRoot, '--env-file', (Join-Path $ProjectRoot '.env'), '-f', (Join-Path $ProjectRoot 'docker-compose.yml'), '-f', (Join-Path $ProjectRoot 'docker-compose.windows.yml')) + $ComposeArguments
    & docker @arguments
    if ($LASTEXITCODE -ne 0) { throw 'Docker Compose failed. Check the message above; your database volumes were not deleted.' }
}
