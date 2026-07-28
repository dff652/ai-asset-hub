param(
    [string]$InstallerPath = (Join-Path $PSScriptRoot "install.ps1")
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

. $InstallerPath

function Assert-Aiah {
    param(
        [bool]$Condition,
        [string]$Message
    )

    if (-not $Condition) {
        throw $Message
    }
}

$testRoot = Join-Path ([IO.Path]::GetTempPath()) (
    "aiah-install-test-{0}" -f [Guid]::NewGuid().ToString("N")
)
$fixtureBinary = Join-Path $testRoot "fixture-aiah.exe"
$fixtureChecksums = Join-Path $testRoot "SHA256SUMS"
$script:DownloadCount = 0

function Get-AiahVersionFromBinary {
    param([string]$Path)

    if (-not (Test-Path -LiteralPath $Path -PathType Leaf)) {
        return $null
    }
    $content = Get-Content -LiteralPath $Path -Raw
    if ($content -match 'VERSION=([0-9A-Za-z.+-]+)') {
        return $Matches[1]
    }
    return $null
}

function Invoke-AiahDownload {
    param(
        [string]$Uri,
        [string]$OutFile
    )

    $script:DownloadCount++
    if ($Uri.EndsWith("/SHA256SUMS")) {
        Copy-Item -LiteralPath $fixtureChecksums -Destination $OutFile
        return
    }
    if ($Uri -match '/aiah_[^/]+\.exe$') {
        Copy-Item -LiteralPath $fixtureBinary -Destination $OutFile
        return
    }
    throw "unexpected download: $Uri"
}

try {
    New-Item -ItemType Directory -Path $testRoot | Out-Null
    Set-Content -LiteralPath $fixtureBinary -NoNewline -Value "VERSION=0.1.1"
    $digest = (Get-FileHash -Algorithm SHA256 -LiteralPath $fixtureBinary).Hash.ToLowerInvariant()
    Set-Content -LiteralPath $fixtureChecksums -Value (
        "$digest  aiah_0.1.1_windows_amd64.exe"
    )

    Assert-Aiah (
        (Get-AiahVersionFromOutput "aiah 0.1.1, commit test") -eq "0.1.1"
    ) "version output parser rejected valid output"
    Assert-Aiah (
        (ConvertTo-AiahArchitecture (
            [Runtime.InteropServices.Architecture]::X64
        )) -eq "amd64"
    ) "X64 architecture mapping failed"
    Assert-Aiah (
        (ConvertTo-AiahArchitecture (
            [Runtime.InteropServices.Architecture]::Arm64
        )) -eq "arm64"
    ) "Arm64 architecture mapping failed"
    $unsupportedFailed = $false
    try {
        ConvertTo-AiahArchitecture ([Runtime.InteropServices.Architecture]::X86)
    } catch {
        $unsupportedFailed = $true
    }
    Assert-Aiah $unsupportedFailed "unsupported architecture was accepted"

    # Successful verified install.
    $installDir = Join-Path $testRoot "success"
    Install-Aiah -Version "0.1.1" -InstallDir $installDir -Architecture "amd64" |
        Out-Null
    $target = Join-Path $installDir "aiah.exe"
    Assert-Aiah (Test-Path -LiteralPath $target -PathType Leaf) "binary was not installed"
    Assert-Aiah (
        (Get-Content -LiteralPath $target -Raw) -eq "VERSION=0.1.1"
    ) "installed binary content differs"

    # Same version performs no download.
    $script:DownloadCount = 0
    Install-Aiah -Version "0.1.1" -InstallDir $installDir -Architecture "amd64" |
        Out-Null
    Assert-Aiah ($script:DownloadCount -eq 0) "idempotent install downloaded files"

    # Checksum failure preserves the old binary.
    $mismatchDir = Join-Path $testRoot "mismatch"
    New-Item -ItemType Directory -Path $mismatchDir | Out-Null
    $mismatchTarget = Join-Path $mismatchDir "aiah.exe"
    Set-Content -LiteralPath $mismatchTarget -NoNewline -Value "VERSION=0.1.0"
    Set-Content -LiteralPath $fixtureChecksums -Value (
        ("0" * 64) + "  aiah_0.1.1_windows_amd64.exe"
    )
    $checksumFailed = $false
    try {
        Install-Aiah -Version "0.1.1" -InstallDir $mismatchDir -Architecture "amd64" |
            Out-Null
    } catch {
        $checksumFailed = $true
    }
    Assert-Aiah $checksumFailed "checksum mismatch was accepted"
    Assert-Aiah (
        (Get-Content -LiteralPath $mismatchTarget -Raw) -eq "VERSION=0.1.0"
    ) "checksum failure changed the installed binary"

    # Duplicate exact checksum entries are ambiguous.
    $validLine = "$digest  aiah_0.1.1_windows_amd64.exe"
    Set-Content -LiteralPath $fixtureChecksums -Value @($validLine, $validLine)
    $duplicateFailed = $false
    try {
        Install-Aiah -Version "0.1.1" -InstallDir $mismatchDir -Architecture "amd64" |
            Out-Null
    } catch {
        $duplicateFailed = $true
    }
    Assert-Aiah $duplicateFailed "duplicate checksum entries were accepted"
    Assert-Aiah (
        (Get-Content -LiteralPath $mismatchTarget -Raw) -eq "VERSION=0.1.0"
    ) "duplicate checksum failure changed the installed binary"

    # Replacement is same-directory and does not delete the target before success.
    $replaceDir = Join-Path $testRoot "replace"
    New-Item -ItemType Directory -Path $replaceDir | Out-Null
    $replaceTarget = Join-Path $replaceDir "aiah.exe"
    $replaceStage = Join-Path $replaceDir ".aiah.install.test.tmp"
    Set-Content -LiteralPath $replaceTarget -NoNewline -Value "old"
    Set-Content -LiteralPath $replaceStage -NoNewline -Value "new"
    Move-AiahIntoPlace $replaceStage $replaceTarget
    Assert-Aiah (
        (Get-Content -LiteralPath $replaceTarget -Raw) -eq "new"
    ) "atomic replacement did not install the stage"

    Set-Content -LiteralPath $replaceTarget -NoNewline -Value "old-again"
    $missingStage = Join-Path $replaceDir ".aiah.install.missing.tmp"
    $replaceFailed = $false
    try {
        Move-AiahIntoPlace $missingStage $replaceTarget
    } catch {
        $replaceFailed = $true
    }
    Assert-Aiah $replaceFailed "missing stage was reported as success"
    Assert-Aiah (
        (Get-Content -LiteralPath $replaceTarget -Raw) -eq "old-again"
    ) "failed replacement removed or changed the old binary"

    Write-Output "install.ps1: OK"
}
finally {
    Remove-Item -LiteralPath $testRoot -Recurse -Force -ErrorAction SilentlyContinue
}
