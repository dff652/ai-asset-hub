# Install a pinned aiah Windows release after verifying its published SHA256.

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"
$script:DefaultAiahVersion = "0.1.1"

function Get-AiahVersionFromOutput {
    param([string]$Output)

    if ($Output -match '^aiah\s+([^,\s]+)') {
        return $Matches[1]
    }
    return $null
}

function Get-AiahVersionFromBinary {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $null
    }
    try {
        $line = & $Path version 2>$null | Select-Object -First 1
        if ($LASTEXITCODE -ne 0) {
            return $null
        }
        return Get-AiahVersionFromOutput ([string]$line)
    }
    catch {
        return $null
    }
}

function ConvertTo-AiahArchitecture {
    param($Architecture)

    switch ($Architecture.ToString()) {
        "X64" { return "amd64" }
        "Arm64" { return "arm64" }
        default { throw "unsupported architecture: $Architecture" }
    }
}

function Resolve-AiahInstallDir {
    param([string]$InstallDir)

    if ([string]::IsNullOrWhiteSpace($InstallDir)) {
        throw "AIAH_INSTALL_DIR must not be empty"
    }
    if ($InstallDir -eq "~") {
        return $HOME
    }
    if ($InstallDir.StartsWith("~/") -or $InstallDir.StartsWith("~\")) {
        return Join-Path $HOME $InstallDir.Substring(2)
    }
    return $InstallDir
}

function Invoke-AiahDownload {
    param(
        [string]$Uri,
        [string]$OutFile
    )

    Invoke-WebRequest -UseBasicParsing -Uri $Uri -OutFile $OutFile
}

function Move-AiahIntoPlace {
    param(
        [string]$Stage,
        [string]$Target
    )

    if (Test-Path -LiteralPath $Target -PathType Leaf) {
        $backup = Join-Path (Split-Path -Parent $Target) (
            ".aiah.replace.{0}.bak" -f [Guid]::NewGuid().ToString("N")
        )
        [IO.File]::Replace($Stage, $Target, $backup, $true)
        Remove-Item -LiteralPath $backup -Force -ErrorAction SilentlyContinue
        return
    }
    [IO.File]::Move($Stage, $Target)
}

function Install-Aiah {
    param(
        [string]$Version,
        [string]$InstallDir,
        [ValidateSet("amd64", "arm64")]
        [string]$Architecture
    )

    $Version = $Version.TrimStart("v")
    if ($Version -notmatch '^[0-9A-Za-z.+-]+$') {
        throw "invalid AIAH_VERSION: $Version"
    }
    $InstallDir = Resolve-AiahInstallDir $InstallDir
    $target = Join-Path $InstallDir "aiah.exe"

    if ((Get-AiahVersionFromBinary $target) -eq $Version) {
        Write-Output "aiah $Version is already installed at $target"
        return
    }

    $asset = "aiah_{0}_windows_{1}.exe" -f $Version, $Architecture
    $releaseBase = "https://github.com/dff652/ai-asset-hub/releases/download/v$Version"
    $tempDir = Join-Path ([IO.Path]::GetTempPath()) (
        "aiah-install-{0}" -f [Guid]::NewGuid().ToString("N")
    )
    $checksums = Join-Path $tempDir "SHA256SUMS"
    $binary = Join-Path $tempDir $asset
    $stage = $null

    try {
        New-Item -ItemType Directory -Path $tempDir | Out-Null
        Invoke-AiahDownload "$releaseBase/SHA256SUMS" $checksums
        Invoke-AiahDownload "$releaseBase/$asset" $binary

        $entries = @()
        foreach ($line in Get-Content -LiteralPath $checksums) {
            if ($line -match '^([0-9A-Fa-f]{64})\s+\*?(.+)$' -and $Matches[2] -ceq $asset) {
                $entries += $Matches[1].ToLowerInvariant()
            }
        }
        if ($entries.Count -ne 1) {
            throw "SHA256SUMS must contain exactly one entry for $asset"
        }
        $expected = $entries[0]
        $actual = (Get-FileHash -Algorithm SHA256 -LiteralPath $binary).Hash.ToLowerInvariant()
        if ($actual -ne $expected) {
            throw "SHA256 verification failed for $asset"
        }
        if ((Get-AiahVersionFromBinary $binary) -ne $Version) {
            throw "$asset did not report version $Version"
        }

        New-Item -ItemType Directory -Force -Path $InstallDir | Out-Null
        $stage = Join-Path $InstallDir (
            ".aiah.install.{0}.tmp" -f [Guid]::NewGuid().ToString("N")
        )
        Copy-Item -LiteralPath $binary -Destination $stage
        Move-AiahIntoPlace $stage $target
        $stage = $null
    }
    finally {
        if ($null -ne $stage) {
            Remove-Item -LiteralPath $stage -Force -ErrorAction SilentlyContinue
        }
        Remove-Item -LiteralPath $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }

    Write-Output "installed aiah $Version to $target"
    $pathEntries = @($env:PATH -split [IO.Path]::PathSeparator)
    if ($InstallDir -notin $pathEntries) {
        Write-Output "hint: add $InstallDir to PATH"
    }
}

function Invoke-AiahInstaller {
    if (-not [Runtime.InteropServices.RuntimeInformation]::IsOSPlatform(
        [Runtime.InteropServices.OSPlatform]::Windows
    )) {
        throw "install.ps1 supports Windows only"
    }

    $version = if (Test-Path Env:AIAH_VERSION) {
        $env:AIAH_VERSION
    }
    else {
        $script:DefaultAiahVersion
    }
    $installDir = if (Test-Path Env:AIAH_INSTALL_DIR) {
        $env:AIAH_INSTALL_DIR
    }
    else {
        Join-Path $HOME ".local\bin"
    }
    $architecture = ConvertTo-AiahArchitecture (
        [Runtime.InteropServices.RuntimeInformation]::OSArchitecture
    )
    Install-Aiah -Version $version -InstallDir $installDir -Architecture $architecture
}

if ($MyInvocation.InvocationName -ne ".") {
    if ($args.Count -ne 0) {
        throw "install.ps1 accepts configuration through AIAH_VERSION and AIAH_INSTALL_DIR"
    }
    Invoke-AiahInstaller
}
