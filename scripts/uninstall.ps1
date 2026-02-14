#Requires -Version 5.1
<#
.SYNOPSIS
    Uninstalls coderaft from Windows.

.DESCRIPTION
    This script removes the coderaft binary and optionally removes the install
    directory from the user's PATH.

.PARAMETER InstallDir
    The directory where coderaft is installed. Defaults to "$env:LOCALAPPDATA\coderaft\bin".

.EXAMPLE
    # Default uninstall
    .\uninstall.ps1

    # Custom install directory
    .\uninstall.ps1 -InstallDir "C:\tools\coderaft"

    # One-liner from GitHub (run in PowerShell):
    irm https://raw.githubusercontent.com/itzcozi/coderaft/main/scripts/uninstall.ps1 | iex
    # or mirror: irm https://coderaft.ar0.eu/uninstall.ps1 | iex
#>

[CmdletBinding()]
param(
    [string]$InstallDir = "$env:LOCALAPPDATA\coderaft\bin"
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# =============================================================================
# Helpers
# =============================================================================

function Write-Header {
    Write-Host ""
    Write-Host " ============ " -ForegroundColor Cyan
    Write-Host "   coderaft   " -ForegroundColor Cyan
    Write-Host "   uninstall  " -ForegroundColor Cyan
    Write-Host " ============ " -ForegroundColor Cyan
    Write-Host ""
}

function Write-Info    { param([string]$Message) Write-Host "  [info]    $Message" -ForegroundColor Blue }
function Write-Ok      { param([string]$Message) Write-Host "  [ok]      $Message" -ForegroundColor Green }
function Write-Warn    { param([string]$Message) Write-Host "  [warn]    $Message" -ForegroundColor Yellow }
function Write-Err     { param([string]$Message) Write-Host "  [error]   $Message" -ForegroundColor Red }

# =============================================================================
# Main
# =============================================================================

Write-Header

$binaryName = "coderaft.exe"
$binaryPath = Join-Path $InstallDir $binaryName

# Check if coderaft is installed
Write-Info "Looking for coderaft installation..."

$foundPath = $null

if (Test-Path $binaryPath) {
    $foundPath = $binaryPath
} else {
    # Check if it's in PATH somewhere else
    $cmd = Get-Command "coderaft" -ErrorAction SilentlyContinue
    if ($cmd) {
        $foundPath = $cmd.Source
        $InstallDir = Split-Path $foundPath -Parent
    }
}

if (-not $foundPath) {
    Write-Warn "coderaft is not installed or could not be found."
    Write-Info "Checked: $binaryPath"
    exit 0
}

Write-Info "Found coderaft at: $foundPath"

# Confirm removal
Write-Host ""
$response = Read-Host "Are you sure you want to uninstall coderaft? [y/N]"
if ($response -notmatch '^[Yy]') {
    Write-Info "Uninstall cancelled."
    exit 0
}

# Remove the binary
Write-Info "Removing coderaft binary..."

try {
    Remove-Item $foundPath -Force
    Write-Ok "Removed $foundPath"
} catch {
    Write-Err "Failed to remove $foundPath : $_"
    Write-Info "Try closing any terminals using coderaft and run again."
    exit 1
}

# Remove empty install directory
if (Test-Path $InstallDir) {
    $remaining = Get-ChildItem $InstallDir -ErrorAction SilentlyContinue
    if (-not $remaining -or $remaining.Count -eq 0) {
        Write-Info "Removing empty install directory..."
        Remove-Item $InstallDir -Force -ErrorAction SilentlyContinue
        Write-Ok "Removed $InstallDir"
    }
}

# Remove from PATH
Write-Info "Checking PATH..."

$userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
if ($userPath -like "*$InstallDir*") {
    Write-Info "Removing $InstallDir from user PATH..."
    $newPath = ($userPath -split ';' | Where-Object { $_ -ne $InstallDir -and $_ -ne '' }) -join ';'
    [Environment]::SetEnvironmentVariable('PATH', $newPath, 'User')
    Write-Ok "Removed from PATH. Changes will apply in new terminals."
} else {
    Write-Ok "$InstallDir is not in user PATH."
}

# Check for PowerShell profile completion
Write-Info "Checking for shell completion in PowerShell profile..."

$profilePath = $PROFILE
if (Test-Path $profilePath) {
    $profileContent = Get-Content $profilePath -Raw -ErrorAction SilentlyContinue
    if ($profileContent -match 'coderaft completion powershell') {
        Write-Warn "Found coderaft completion in your PowerShell profile."
        Write-Info "You may want to manually remove it from: $profilePath"
    }
}

Write-Host ""
Write-Ok "coderaft has been uninstalled successfully!"
Write-Host ""
Write-Info "Note: This script does not remove:"
Write-Host "  - Docker Desktop (you may still need it for other tools)"
Write-Host "  - Go or Git (you may still need them)"
Write-Host "  - Any coderaft projects you created"
Write-Host ""
Write-Info "To remove Docker containers created by coderaft:"
Write-Host "  docker ps -a | Select-String coderaft | ForEach-Object { docker rm -f (`$_ -split '\s+')[0] }"
Write-Host ""
Write-Info "To remove Docker images created by coderaft:"
Write-Host "  docker images | Select-String coderaft | ForEach-Object { docker rmi -f (`$_ -split '\s+')[2] }"
Write-Host ""
