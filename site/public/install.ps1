# ---------------------------------------------------------------------------
# FormSpec installer (PowerShell) — Windows amd64 & arm64.
#
#   irm https://formspec.dev/install.ps1 | iex
#
# Opsi:
#   .\install.ps1                        # install versi terbaru (stable)
#   .\install.ps1 -Version v0.0.5       # install versi tertentu
#   $env:FORMSPEC_VERSION = 'v0.0.5'; .\install.ps1
#
# Perilaku:
#   - Tanpa admin: binary dipasang di %LOCALAPPDATA%\Programs\formspec (user-local).
#   - Deteksi arsitektur otomatis (AMD64/ARM64); download zip dari GitHub Releases.
#   - Menambahkan folder install ke user PATH bila belum ada.
#   - Idempotent: jalankan ulang = upgrade/overwrite.
# ---------------------------------------------------------------------------
[CmdletBinding()]
param(
  [string]$Version = $env:FORMSPEC_VERSION
)

$ErrorActionPreference = 'Stop'
$Repo = 'primadi/formspec'
$InstallDir = Join-Path $env:LOCALAPPDATA 'Programs\formspec'

function Log($msg) { Write-Host ("[OK] " + $msg) -ForegroundColor Green }
function Warn($msg) { Write-Host ("[!] " + $msg) -ForegroundColor Yellow }

# --- Deteksi arsitektur -------------------------------------------------------
switch ($env:PROCESSOR_ARCHITECTURE) {
  'AMD64' { $Arch = 'amd64' }
  'ARM64' { $Arch = 'arm64' }
  default { throw "Arsitektur tidak didukung: $env:PROCESSOR_ARCHITECTURE" }
}

# --- Resolusi versi ------------------------------------------------------------
if (-not $Version) {
  Log "Mencari versi terbaru..."
  $Release = Invoke-RestMethod -Uri "https://api.github.com/repos/$Repo/releases/latest"
  $Version = $Release.tag_name
  if (-not $Version) { throw "Gagal menentukan versi terbaru. Coba: .\install.ps1 -Version vX.Y.Z" }
}
Log "Versi: $Version"

# --- Download arsip --------------------------------------------------------------
$Archive = "formspec-windows-$Arch.zip"
$Url = "https://github.com/$Repo/releases/download/$Version/$Archive"
$Tmp = Join-Path ([System.IO.Path]::GetTempPath()) ("formspec-install-" + [guid]::NewGuid())
New-Item -ItemType Directory -Path $Tmp -Force | Out-Null
try {
  Log "Mengunduh $Archive..."
  $ZipPath = Join-Path $Tmp $Archive
  Invoke-WebRequest -Uri $Url -OutFile $ZipPath

  # --- Verifikasi checksum bila SHA256SUMS tersedia --------------------------------
  try {
    $SumsPath = Join-Path $Tmp 'SHA256SUMS.txt'
    Invoke-WebRequest -Uri "https://github.com/$Repo/releases/download/$Version/SHA256SUMS.txt" -OutFile $SumsPath
    $Expected = (Select-String -Path $SumsPath -Pattern ([regex]::Escape($Archive))).Line.Split(' ')[0]
    if ($Expected) {
      $Actual = (Get-FileHash -Algorithm SHA256 $ZipPath).Hash.ToLower()
      if ($Actual -ne $Expected.ToLower()) {
        throw "Checksum tidak cocok! expected=$Expected actual=$Actual"
      }
      Log "Checksum SHA256 OK"
    }
  } catch [System.Net.WebException] { } # SHA256SUMS opsional

  # --- Ekstrak + install -------------------------------------------------------------
  Log "Mengekstrak dan memasang ke $InstallDir..."
  Expand-Archive -Path $ZipPath -DestinationPath $Tmp -Force
  $Exe = Join-Path $Tmp 'formspec.exe'
  if (-not (Test-Path $Exe)) { throw "Zip tidak berisi formspec.exe" }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  Copy-Item $Exe (Join-Path $InstallDir 'formspec.exe') -Force
  Log "Terpasang di $InstallDir\formspec.exe"
} finally {
  Remove-Item -Recurse -Force $Tmp -ErrorAction SilentlyContinue
}

# --- Set user PATH bila belum ada ------------------------------------------------
$UserPath = [Environment]::GetEnvironmentVariable('Path', 'User')
$PathParts = $UserPath -split ';' | Where-Object { $_ }
if ($PathParts -notcontains $InstallDir) {
  Log "Menambahkan $InstallDir ke user PATH..."
  [Environment]::SetEnvironmentVariable('Path', (($PathParts + $InstallDir) -join ';'), 'User')
  # Aktifkan juga untuk sesi ini
  $env:Path = "$env:Path;$InstallDir"
  Warn "Buka terminal baru agar PATH baru berlaku di semua jendela."
}

# --- Verifikasi --------------------------------------------------------------------
& (Join-Path $InstallDir 'formspec.exe') version | Write-Host

Write-Host ""
Write-Host "Next steps: https://docs.formspec.dev/guides/install.html"
