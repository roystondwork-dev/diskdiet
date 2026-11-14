# Installs diskdiet (Windows).
#
#   irm https://roystondwork-dev.github.io/diskdiet/install.ps1 | iex
#
# Downloads the latest release for this CPU from GitHub, checks it against the
# release's checksums, installs diskdiet.exe to %LOCALAPPDATA%\Programs\diskdiet
# (or $env:DISKDIET_INSTALL_DIR) and adds that folder to your user PATH.
#
# Options (environment variables):
#   DISKDIET_INSTALL_DIR  install here instead
#   DISKDIET_VERSION      a version such as 0.6.3 instead of the latest
$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$repo = 'roystondwork-dev/diskdiet'

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "diskdiet installer: unsupported CPU $($env:PROCESSOR_ARCHITECTURE)" }
}

if ($env:DISKDIET_VERSION) {
    $base = "https://github.com/$repo/releases/download/v$($env:DISKDIET_VERSION.TrimStart('v'))"
} else {
    # No API call (rate-limited per IP): the latest release's checksums name
    # its archives, which carry the version.
    $base = "https://github.com/$repo/releases/latest/download"
}

$tmp = Join-Path ([IO.Path]::GetTempPath()) ("diskdiet-" + [Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null
try {
    # Saved to a file: GitHub serves it as binary, which Content would return as bytes.
    Invoke-WebRequest "$base/checksums.txt" -OutFile (Join-Path $tmp 'checksums.txt') -UseBasicParsing
    $line = Get-Content (Join-Path $tmp 'checksums.txt') | Where-Object { $_ -match "\s\*?(diskdiet_(.+)_windows_${arch}\.zip)\s*$" } | Select-Object -First 1
    if (-not $line) { throw "diskdiet installer: no diskdiet build for windows/$arch in this release" }
    $null = $line -match "\s\*?(diskdiet_(.+)_windows_${arch}\.zip)\s*$"
    $archive = $Matches[1]
    $version = $Matches[2]
    $want = ($line -split '\s+')[0]
    Write-Host "Downloading diskdiet $version for windows/$arch..."
    Invoke-WebRequest "$base/$archive" -OutFile (Join-Path $tmp $archive) -UseBasicParsing
    $got = (Get-FileHash (Join-Path $tmp $archive) -Algorithm SHA256).Hash.ToLower()
    if (-not $want -or $want.ToLower() -ne $got) { throw "diskdiet installer: checksum mismatch for $archive" }
    Expand-Archive (Join-Path $tmp $archive) -DestinationPath (Join-Path $tmp 'x') -Force

    $dir = $env:DISKDIET_INSTALL_DIR
    if (-not $dir) { $dir = Join-Path $env:LOCALAPPDATA 'Programs\diskdiet' }
    New-Item -ItemType Directory -Path $dir -Force | Out-Null
    $exe = Join-Path $dir 'diskdiet.exe'
    Copy-Item (Join-Path $tmp 'x\diskdiet.exe') $exe -Force
    Write-Host "Installed $exe"
} finally {
    Remove-Item $tmp -Recurse -Force -ErrorAction SilentlyContinue
}

# Add the folder to the user PATH (new terminals pick it up).
$userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
if (-not (($userPath -split ';') -contains $dir)) {
    [Environment]::SetEnvironmentVariable('Path', ($(if ($userPath) { "$userPath;" } else { '' }) + $dir), 'User')
    Write-Host "Added $dir to your PATH (open a new terminal to use 'diskdiet')."
}
$env:Path = "$env:Path;$dir"
& $exe version
