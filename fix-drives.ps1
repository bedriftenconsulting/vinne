# Fix External Drives Script
# Run as Administrator in PowerShell

Write-Host "=== External Drive Diagnostic & Fix ===" -ForegroundColor Cyan

# Check all disks
Write-Host "`n[1] Listing all disks..." -ForegroundColor Yellow
Get-Disk | Format-Table Number, FriendlyName, OperationalStatus, HealthStatus, Size -AutoSize

# Check all volumes/partitions
Write-Host "`n[2] Listing all volumes..." -ForegroundColor Yellow
Get-Volume | Format-Table DriveLetter, FileSystemLabel, FileSystem, DriveType, HealthStatus, OperationalStatus, SizeRemaining, Size -AutoSize

# Try to bring offline disks online
Write-Host "`n[3] Bringing any offline disks online..." -ForegroundColor Yellow
$offlineDisks = Get-Disk | Where-Object { $_.OperationalStatus -eq 'Offline' }
if ($offlineDisks) {
    foreach ($disk in $offlineDisks) {
        Write-Host "  Bringing Disk $($disk.Number) online..." -ForegroundColor Green
        Set-Disk -Number $disk.Number -IsOffline $false
        Set-Disk -Number $disk.Number -IsReadOnly $false
    }
} else {
    Write-Host "  No offline disks found." -ForegroundColor Gray
}

# Run CHKDSK on D and F if they exist
foreach ($letter in @("D", "F")) {
    $vol = Get-Volume -DriveLetter $letter -ErrorAction SilentlyContinue
    if ($vol) {
        Write-Host "`n[4] Running CHKDSK on ${letter}:..." -ForegroundColor Yellow
        Repair-Volume -DriveLetter $letter -Scan
    } else {
        Write-Host "`n[4] Drive ${letter}: not found or not accessible." -ForegroundColor Red
    }
}

# Update USB/disk drivers
Write-Host "`n[5] Scanning for hardware changes..." -ForegroundColor Yellow
pnputil /scan-devices

Write-Host "`n=== Done! Check above for errors ===" -ForegroundColor Cyan
Write-Host "If drives still don't show, try a different USB port or cable." -ForegroundColor White
