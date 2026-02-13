#Requires -Version 5.1
<#
.SYNOPSIS
    Build script for coderaft on Windows (replaces Makefile).

.DESCRIPTION
    Provides build, test, install, clean, and cross-compile targets for
    coderaft on Windows without requiring 'make'.

.PARAMETER Target
    The build target. Defaults to "build".
    Valid targets: build, dev, test, clean, install, build-all, deps, fmt, help

.PARAMETER Install
    Shortcut for -Target install. Builds and installs coderaft.

.EXAMPLE
    .\build.ps1                  # Build for current platform
    .\build.ps1 -Target test     # Run tests
    .\build.ps1 -Target build-all # Cross-compile for all platforms
    .\build.ps1 -Install         # Build and install
    .\build.ps1 -Target clean    # Clean build artifacts
#>

[CmdletBinding()]
param(
    [ValidateSet("build", "dev", "test", "clean", "install", "build-all",
                 "build-linux", "build-linux-arm64", "build-darwin", "build-darwin-arm64",
                 "build-windows", "build-windows-arm64",
                 "deps", "fmt", "help")]
    [string]$Target = "build",
    [switch]$Install
)

$ErrorActionPreference = 'Stop'

# =============================================================================
# Configuration
# =============================================================================

$Version = "1.0"
$BinaryName = "coderaft"
$BinaryPath = ".\cmd\coderaft"
$BuildDir = ".\build"

try {
    $GitCommit = (git rev-parse --short HEAD 2>$null)
    if ($LASTEXITCODE -ne 0) { $GitCommit = "unknown" }
} catch {
    $GitCommit = "unknown"
}

$LdFlags = "-X coderaft/internal/commands.Version=$Version -X coderaft/internal/commands.CommitHash=$GitCommit"

# =============================================================================
# Helpers
# =============================================================================

function Ensure-BuildDir {
    if (-not (Test-Path $BuildDir)) {
        New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null
    }
}

function Write-Step { param([string]$Message) Write-Host "  -> $Message" -ForegroundColor Cyan }

# =============================================================================
# Targets
# =============================================================================

function Invoke-Build {
    Write-Host "Building $BinaryName version $Version (commit $GitCommit)..." -ForegroundColor Green
    Ensure-BuildDir
    $env:CGO_ENABLED = "0"
    go build -ldflags $LdFlags -o "$BuildDir\$BinaryName.exe" $BinaryPath
    if ($LASTEXITCODE -ne 0) { throw "Build failed" }
    Write-Host "Built $BuildDir\$BinaryName.exe" -ForegroundColor Green
}

function Invoke-Dev {
    Write-Host "Building $BinaryName for development..." -ForegroundColor Green
    Ensure-BuildDir
    go build -o "$BuildDir\$BinaryName.exe" $BinaryPath
    if ($LASTEXITCODE -ne 0) { throw "Build failed" }
    Write-Host "Built $BuildDir\$BinaryName.exe" -ForegroundColor Green
}

function Invoke-Test {
    Write-Host "Running tests..." -ForegroundColor Green
    go test -v ./...
}

function Invoke-Clean {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Yellow
    go clean
    if (Test-Path $BuildDir) {
        Remove-Item -Recurse -Force $BuildDir
    }
    Write-Host "Clean complete." -ForegroundColor Green
}

function Invoke-Install {
    Invoke-Build

    $installDir = "$env:LOCALAPPDATA\coderaft\bin"
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }

    Copy-Item "$BuildDir\$BinaryName.exe" "$installDir\$BinaryName.exe" -Force
    Write-Host "Installed to $installDir\$BinaryName.exe" -ForegroundColor Green

    # Add to PATH if needed
    $userPath = [Environment]::GetEnvironmentVariable('PATH', 'User')
    if ($userPath -notlike "*$installDir*") {
        Write-Host "Adding $installDir to user PATH..." -ForegroundColor Cyan
        [Environment]::SetEnvironmentVariable('PATH', "$userPath;$installDir", 'User')
        $env:PATH = "$env:PATH;$installDir"
        Write-Host "Added to PATH. New terminals will pick this up automatically." -ForegroundColor Green
    }
}

function Invoke-Deps {
    Write-Host "Downloading dependencies..." -ForegroundColor Green
    go mod download
    go mod tidy
}

function Invoke-Fmt {
    Write-Host "Formatting code..." -ForegroundColor Green
    go fmt ./...
}

function Invoke-CrossBuild {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$Suffix
    )
    Write-Step "Building $BinaryName for $GOOS/$GOARCH..."
    Ensure-BuildDir
    $env:CGO_ENABLED = "0"
    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    go build -ldflags $LdFlags -o "$BuildDir\$BinaryName-$Suffix" $BinaryPath
    if ($LASTEXITCODE -ne 0) { throw "Build failed for $GOOS/$GOARCH" }
    # Reset env
    Remove-Item env:GOOS -ErrorAction SilentlyContinue
    Remove-Item env:GOARCH -ErrorAction SilentlyContinue
    Write-Step "Built $BuildDir\$BinaryName-$Suffix"
}

function Invoke-BuildAll {
    Write-Host "Building $BinaryName for all platforms..." -ForegroundColor Green
    Invoke-CrossBuild -GOOS "linux"   -GOARCH "amd64" -Suffix "linux-amd64"
    Invoke-CrossBuild -GOOS "linux"   -GOARCH "arm64" -Suffix "linux-arm64"
    Invoke-CrossBuild -GOOS "darwin"  -GOARCH "amd64" -Suffix "darwin-amd64"
    Invoke-CrossBuild -GOOS "darwin"  -GOARCH "arm64" -Suffix "darwin-arm64"
    Invoke-CrossBuild -GOOS "windows" -GOARCH "amd64" -Suffix "windows-amd64.exe"
    Invoke-CrossBuild -GOOS "windows" -GOARCH "arm64" -Suffix "windows-arm64.exe"
    Write-Host "All platform builds complete." -ForegroundColor Green
}

function Invoke-Help {
    Write-Host ""
    Write-Host "coderaft build script (Windows)" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Usage: .\build.ps1 [-Target <target>] [-Install]" -ForegroundColor White
    Write-Host ""
    Write-Host "Targets:" -ForegroundColor Yellow
    Write-Host "  build              Build binary for current platform (default)"
    Write-Host "  dev                Build for development (no optimizations)"
    Write-Host "  test               Run tests"
    Write-Host "  clean              Clean build artifacts"
    Write-Host "  install            Build and install to PATH"
    Write-Host "  build-all          Cross-compile for all platforms"
    Write-Host "  build-linux        Build for Linux AMD64"
    Write-Host "  build-linux-arm64  Build for Linux ARM64"
    Write-Host "  build-darwin       Build for macOS AMD64"
    Write-Host "  build-darwin-arm64 Build for macOS ARM64"
    Write-Host "  build-windows      Build for Windows AMD64"
    Write-Host "  build-windows-arm64  Build for Windows ARM64"
    Write-Host "  deps               Download and tidy dependencies"
    Write-Host "  fmt                Format code"
    Write-Host "  help               Show this help message"
    Write-Host ""
}

# =============================================================================
# Main
# =============================================================================

if ($Install) { $Target = "install" }

switch ($Target) {
    "build"              { Invoke-Build }
    "dev"                { Invoke-Dev }
    "test"               { Invoke-Test }
    "clean"              { Invoke-Clean }
    "install"            { Invoke-Install }
    "build-all"          { Invoke-BuildAll }
    "build-linux"        { Invoke-CrossBuild -GOOS "linux"   -GOARCH "amd64" -Suffix "linux-amd64" }
    "build-linux-arm64"  { Invoke-CrossBuild -GOOS "linux"   -GOARCH "arm64" -Suffix "linux-arm64" }
    "build-darwin"       { Invoke-CrossBuild -GOOS "darwin"  -GOARCH "amd64" -Suffix "darwin-amd64" }
    "build-darwin-arm64" { Invoke-CrossBuild -GOOS "darwin"  -GOARCH "arm64" -Suffix "darwin-arm64" }
    "build-windows"      { Invoke-CrossBuild -GOOS "windows" -GOARCH "amd64" -Suffix "windows-amd64.exe" }
    "build-windows-arm64"{ Invoke-CrossBuild -GOOS "windows" -GOARCH "arm64" -Suffix "windows-arm64.exe" }
    "deps"               { Invoke-Deps }
    "fmt"                { Invoke-Fmt }
    "help"               { Invoke-Help }
}
