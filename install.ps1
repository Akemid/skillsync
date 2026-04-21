#Requires -Version 5.1
$ErrorActionPreference = 'Stop'

$Repo = "Akemid/skillsync"
$BinaryName = "skillsync.exe"

# Detect architecture
$Arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    "AMD64" { "amd64" }
    "ARM64" { "arm64" }
    default {
        Write-Error "Unsupported architecture: $env:PROCESSOR_ARCHITECTURE"
        exit 1
    }
}

# Get latest version from GitHub API
try {
    $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest" -UseBasicParsing
    $Version = $Release.tag_name
} catch {
    Write-Error "Could not determine latest version: $_"
    exit 1
}

$VersionNoV = $Version.TrimStart('v')
$Archive = "skillsync_${VersionNoV}_windows_${Arch}.zip"
$BaseUrl = "https://github.com/$Repo/releases/download/$Version"
$ArchiveUrl = "$BaseUrl/$Archive"

# Download to temp dir
$TmpDir = Join-Path $env:TEMP "skillsync-install-$(Get-Random)"
New-Item -ItemType Directory -Path $TmpDir | Out-Null

try {
    Write-Host "Downloading skillsync $Version (windows/$Arch)..."
    $ArchivePath = Join-Path $TmpDir $Archive
    Invoke-WebRequest -Uri $ArchiveUrl -OutFile $ArchivePath -UseBasicParsing

    # Extract
    Expand-Archive -Path $ArchivePath -DestinationPath $TmpDir -Force

    # Install destination
    $InstallDir = Join-Path $env:LOCALAPPDATA "Programs\skillsync"
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null

    $ExtractedBinary = Join-Path $TmpDir "skillsync.exe"
    Copy-Item -Path $ExtractedBinary -Destination (Join-Path $InstallDir $BinaryName) -Force

    # Add to user PATH if not already present
    $UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($UserPath -notlike "*$InstallDir*") {
        [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
        Write-Host ""
        Write-Host "Note: $InstallDir added to your PATH."
        Write-Host "Restart your terminal for changes to take effect."
    }

    Write-Host ""
    Write-Host "skillsync $Version installed successfully -> $InstallDir\$BinaryName"
    Write-Host ""

    # Warn about symlinks
    $DevMode = (Get-ItemProperty -Path "HKLM:\SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock" -ErrorAction SilentlyContinue).AllowDevelopmentWithoutDevLicense
    if ($DevMode -ne 1) {
        Write-Host "Warning: skillsync uses symlinks. For full functionality, enable Developer Mode:"
        Write-Host "  Settings -> Privacy & Security -> For developers -> Developer Mode"
        Write-Host "  Or run skillsync as Administrator."
    }

} finally {
    Remove-Item -Recurse -Force $TmpDir -ErrorAction SilentlyContinue
}
