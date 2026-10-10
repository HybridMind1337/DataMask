# DataMask one-liner installer for Windows PowerShell
# Usage: iwr -useb https://raw.githubusercontent.com/HybridMind1337/DataMask/main/scripts/install.ps1 | iex

$ErrorActionPreference = "Stop"

$repo = "HybridMind1337/DataMask"
$binName = "datamask.exe"
$installDir = "$env:LOCALAPPDATA\DataMask\bin"

Write-Host "=> Checking system architecture..." -ForegroundColor Cyan
$arch = if ([System.Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }
if ($env:PROCESSOR_ARCHITECTURE -eq "ARM64") {
    $arch = "arm64"
}

Write-Host "=> Fetching latest release metadata..." -ForegroundColor Cyan
$releaseUrl = "https://api.github.com/repos/$repo/releases/latest"
try {
    $release = Invoke-RestMethod -Uri $releaseUrl -Headers @{ "User-Agent" = "DataMask-Installer" }
    $tag = $release.tag_name
} catch {
    $tag = "v1.0.0"
}

$version = $tag.TrimStart("v")
$zipName = "datamask_${version}_windows_${arch}.zip"
$downloadUrl = "https://github.com/$repo/releases/download/$tag/$zipName"

$tmpDir = Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmpDir -Force | Out-Null

try {
    Write-Host "=> Downloading $downloadUrl..." -ForegroundColor Cyan
    $zipPath = Join-Path $tmpDir $zipName
    Invoke-WebRequest -Uri $downloadUrl -OutFile $zipPath

    Write-Host "=> Extracting archive..." -ForegroundColor Cyan
    Expand-Archive -Path $zipPath -DestinationPath $tmpDir -Force

    New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    Copy-Item -Path (Join-Path $tmpDir $binName) -Destination (Join-Path $installDir $binName) -Force

    # Add to user PATH if not present
    $userPath = [System.Environment]::GetEnvironmentVariable("Path", [System.EnvironmentVariableTarget]::User)
    if ($userPath -notlike "*$installDir*") {
        Write-Host "=> Adding $installDir to User PATH..." -ForegroundColor Cyan
        [System.Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", [System.EnvironmentVariableTarget]::User)
        $env:Path = "$env:Path;$installDir"
    }

    Write-Host ""
    Write-Host "==================================================" -ForegroundColor Green
    Write-Host " DataMask successfully installed! ($tag)" -ForegroundColor Green
    Write-Host " Binary location: $installDir\$binName" -ForegroundColor Green
    Write-Host " Run 'datamask --help' or 'datamask wizard' to start." -ForegroundColor Green
    Write-Host "==================================================" -ForegroundColor Green
} finally {
    Remove-Item -Path $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
}
