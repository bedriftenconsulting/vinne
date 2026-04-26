# Automated deployment script for PowerShell
# Usage: .\deploy-auto.ps1

$VM_USER = "suraj"
$VM_IP = "34.42.87.251"
$SSH_KEY = "C:\Users\Suraj\.ssh\google_compute_engine"
$TIMESTAMP = Get-Date -Format "yyyyMMddHHmmss"

Write-Host "🚀 Starting automated deployment..." -ForegroundColor Green

# Build website
Write-Host "📦 Building website..." -ForegroundColor Yellow
Set-Location vinne-website
npm run build

# Get built file names
$JS_FILE = Get-ChildItem "dist/assets/index-*.js" | Select-Object -First 1
$CSS_FILE = Get-ChildItem "dist/assets/index-*.css" | Select-Object -First 1
$LOGO_FILE = Get-ChildItem "dist/assets/logo-*.png" | Select-Object -First 1

if (-not $JS_FILE -or -not $CSS_FILE) {
    Write-Host "❌ Build files not found!" -ForegroundColor Red
    exit 1
}

$JS_NAME = $JS_FILE.Name
$CSS_NAME = $CSS_FILE.Name
$LOGO_NAME = $LOGO_FILE.Name

Write-Host "📁 Built files:" -ForegroundColor Cyan
Write-Host "  - $JS_NAME"
Write-Host "  - $CSS_NAME"
Write-Host "  - $LOGO_NAME"

# Upload files
Write-Host "⬆️  Uploading files..." -ForegroundColor Yellow
scp -i $SSH_KEY -o StrictHostKeyChecking=no $JS_FILE.FullName "${VM_USER}@${VM_IP}:/tmp/"
scp -i $SSH_KEY -o StrictHostKeyChecking=no $CSS_FILE.FullName "${VM_USER}@${VM_IP}:/tmp/"
scp -i $SSH_KEY -o StrictHostKeyChecking=no $LOGO_FILE.FullName "${VM_USER}@${VM_IP}:/tmp/"
scp -i $SSH_KEY -o StrictHostKeyChecking=no "dist/index.html" "${VM_USER}@${VM_IP}:/tmp/"

# Deploy on server
Write-Host "🔄 Deploying on server..." -ForegroundColor Yellow
$deployScript = @"
# Create backup
sudo cp -r /var/www/winbig /var/www/winbig-backup-$TIMESTAMP

# Clear old assets
sudo rm -f /var/www/winbig/assets/index-*.js
sudo rm -f /var/www/winbig/assets/index-*.css
sudo rm -f /var/www/winbig/assets/logo-*.png

# Move new files
sudo cp /tmp/$JS_NAME /var/www/winbig/assets/
sudo cp /tmp/$CSS_NAME /var/www/winbig/assets/
sudo cp /tmp/$LOGO_NAME /var/www/winbig/assets/
sudo cp /tmp/index.html /var/www/winbig/

# Set permissions
sudo chown -R www-data:www-data /var/www/winbig/
sudo chmod -R 755 /var/www/winbig/

# Reload nginx
sudo nginx -t && sudo systemctl reload nginx

echo '✅ Server deployment completed!'
"@

ssh -i $SSH_KEY -o StrictHostKeyChecking=no "${VM_USER}@${VM_IP}" $deployScript

Write-Host "✅ Deployment successful! 🎉" -ForegroundColor Green
Write-Host "🌐 Website: https://winbig.bedriften.xyz" -ForegroundColor Cyan

Set-Location ..