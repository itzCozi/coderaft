#Requires -Version 5.1
<#
.SYNOPSIS
    Installs coderaft on Windows.

.DESCRIPTION
    This script downloads (or builds) the coderaft binary and installs it
    to a directory on the user's PATH. It also verifies that Docker Desktop
    is available.

.PARAMETER InstallDir
    The directory to install coderaft into. Defaults to "$env:LOCALAPPDATA\coderaft\bin".

.EXAMPLE
    # Default install
    .\install.ps1

    # Custom install directory
    .\install.ps1 -InstallDir "C:\tools\coderaft"

    # One-liner from GitHub (run in PowerShell as Administrator):
    irm https://raw.githubusercontent.com/itzcozi/coderaft/main/install.ps1 | iex
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
    Write-Host " ============ " -ForegroundColor Cyan
    Write-Host ""
}

function Write-Info    { param([string]$Message) Write-Host "  [info]    $Message" -ForegroundColor Blue }
function Write-Ok      { param([string]$Message) Write-Host "  [ok]      $Message" -ForegroundColor Green }
function Write-Warn    { param([string]$Message) Write-Host "  [warn]    $Message" -ForegroundColor Yellow }
function Write-Err     { param([string]$Message) Write-Host "  [error]   $Message" -ForegroundColor Red }

function Test-CommandExists {
    param([string]$Command)
    $null = Get-Command $Command -ErrorAction SilentlyContinue
    return $?
}

# =============================================================================
# Pre-flight checks
# =============================================================================

Write-Header

# Check architecture
$arch = if ([Environment]::Is64BitOperatingSystem) {
    if ($env:PROCESSOR_ARCHITECTURE -eq 'ARM64') { 'arm64' } else { 'amd64' }
} else {
    Write-Err "coderaft requires a 64-bit version of Windows."
    exit 1
}
Write-Info "Detected architecture: windows/$arch"

# Check for Docker
Write-Info "Checking for Docker..."
if (Test-CommandExists 'docker') {
    try {
        $dockerVersion = docker version --format '{{.Client.Version}}' 2>$null
        Write-Ok "Docker found: $dockerVersion"
    } catch {
        Write-Warn "Docker CLI found but daemon may not be running."
        Write-Warn "Please ensure Docker Desktop is started before using coderaft."
    }
} else {
    Write-Err "Docker is not installed or not in PATH."
    Write-Host ""
    Write-Info "Please install Docker Desktop for Windows:"
    Write-Info "  https://docs.docker.com/desktop/install/windows-install/"
    Write-Host ""
    Write-Info "After installing Docker Desktop, run this script again."
    exit 1
}

# Check for Git (needed if building from source)
$hasGit = Test-CommandExists 'git'
$hasGo  = Test-CommandExists 'go'

# =============================================================================
# Determine install method
# =============================================================================

$binaryName = "coderaft.exe"
$tempDir = Join-Path ([System.IO.Path]::GetTempPath()) "coderaft-install-$(Get-Random)"
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null

try {
    # Try downloading a pre-built release first
    $releaseBinary = "coderaft-windows-$arch.exe"
    $downloadUrl = "https://github.com/itzcozi/coderaft/releases/latest/download/$releaseBinary"

    Write-Info "Attempting to download pre-built binary..."
    try {
        $webClient = New-Object System.Net.WebClient
        $webClient.Headers.Add("User-Agent", "coderaft-installer/1.0")
        $destPath = Join-Path $tempDir $binaryName
        $webClient.DownloadFile($downloadUrl, $destPath)
        Write-Ok "Downloaded pre-built binary."
    } catch {
        Write-Warn "Pre-built binary not available. Building from source..."

        if (-not $hasGit) {
            Write-Err "Git is required to build from source. Please install Git:"
            Write-Info "  https://git-scm.com/download/win"
            exit 1
        }
        if (-not $hasGo) {
            Write-Err "Go is required to build from source. Please install Go:"
            Write-Info "  https://go.dev/dl/"
            exit 1
        }

        Write-Info "Cloning coderaft repository..."
        git clone --depth 1 https://github.com/itzcozi/coderaft.git (Join-Path $tempDir "coderaft-src") 2>&1 | Out-Null
        if ($LASTEXITCODE -ne 0) {
            Write-Err "Failed to clone repository."
            exit 1
        }

        Push-Location (Join-Path $tempDir "coderaft-src")
        try {
            Write-Info "Building coderaft..."
            $env:CGO_ENABLED = "0"
            $env:GOOS = "windows"
            $env:GOARCH = $arch
            go build -o (Join-Path $tempDir $binaryName) ./cmd/coderaft
            if ($LASTEXITCODE -ne 0) {
                Write-Err "Build failed."
                exit 1
            }
            Write-Ok "Build succeeded."
        } finally {
            Pop-Location
        }
    }

    # =============================================================================
    # Install binary
    # =============================================================================

    Write-Info "Installing to $InstallDir..."
    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    $finalPath = Join-Path $InstallDir $binaryName
    Copy-Item (Join-Path $tempDir $binaryName) $finalPath -Force
    Write-Ok "Installed $finalPath"

    # =============================================================================
    # Add to PATH if needed
    # =============================================================================

    $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($userPath -notlike "*$InstallDir*") {
        Write-Info "Adding $InstallDir to user PATH..."
        [Environment]::SetEnvironmentVariable('PATH', "$userPath;$InstallDir", 'User')
        $env:PATH = "$env:PATH;$InstallDir"
        Write-Ok "Added to PATH. New terminals will pick this up automatically."
    } else {
        Write-Ok "$InstallDir is already in PATH."
    }

    # =============================================================================
    # Verify
    # =============================================================================

    Write-Info "Verifying installation..."
    try {
        $version = & $finalPath version 2>&1
        Write-Ok "coderaft installed successfully: $version"
    } catch {
        Write-Warn "Binary installed but verification failed. Try opening a new terminal."
    }

    # =============================================================================
    # Next steps
    # =============================================================================

    Write-Host ""
    Write-Ok "coderaft installation complete!"
    Write-Host ""
    Write-Info "Next steps:"
    Write-Host "  1. Make sure Docker Desktop is running"
    Write-Host "  2. Open a NEW terminal (PowerShell or CMD)"
    Write-Host "  3. Create your first project:"
    Write-Host "       coderaft init myproject"
    Write-Host "  4. Enter the development environment:"
    Write-Host "       coderaft shell myproject"
    Write-Host "  5. Get help anytime:"
    Write-Host "       coderaft --help"
    Write-Host ""
    Write-Info "Shell completion (optional):"
    Write-Host "  coderaft completion powershell >> `$PROFILE"
    Write-Host ""
    Write-Info "For more information: https://github.com/itzcozi/coderaft"
    Write-Host ""

} finally {
    # Cleanup
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}
