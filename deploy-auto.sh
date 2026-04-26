#!/bin/bash
# Automated deployment script with proper cache busting
# Usage: bash deploy-auto.sh

set -e

VM_USER="suraj"
VM_IP="34.42.87.251"
SSH_KEY="$HOME/.ssh/google_compute_engine"
TIMESTAMP=$(date +%s)

echo "🚀 Starting automated deployment..."

# Build with timestamp
echo "📦 Building website..."
cd vinne-website
npm run build

# Get the actual built file names
JS_FILE=$(ls dist/assets/index-*.js | head -1)
CSS_FILE=$(ls dist/assets/index-*.css | head -1)
LOGO_FILE=$(ls dist/assets/logo-*.png | head -1)

if [ ! -f "$JS_FILE" ] || [ ! -f "$CSS_FILE" ]; then
    echo "❌ Build files not found!"
    exit 1
fi

JS_NAME=$(basename "$JS_FILE")
CSS_NAME=$(basename "$CSS_FILE")
LOGO_NAME=$(basename "$LOGO_FILE")

echo "📁 Built files:"
echo "  - $JS_NAME"
echo "  - $CSS_NAME"
echo "  - $LOGO_NAME"

# Upload files
echo "⬆️  Uploading files..."
scp -i "$SSH_KEY" -o StrictHostKeyChecking=no "$JS_FILE" "$VM_USER@$VM_IP:/tmp/"
scp -i "$SSH_KEY" -o StrictHostKeyChecking=no "$CSS_FILE" "$VM_USER@$VM_IP:/tmp/"
scp -i "$SSH_KEY" -o StrictHostKeyChecking=no "$LOGO_FILE" "$VM_USER@$VM_IP:/tmp/"
scp -i "$SSH_KEY" -o StrictHostKeyChecking=no "dist/index.html" "$VM_USER@$VM_IP:/tmp/"

# Deploy on server
echo "🔄 Deploying on server..."
ssh -i "$SSH_KEY" -o StrictHostKeyChecking=no "$VM_USER@$VM_IP" "
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
"

# Verify deployment
echo "🔍 Verifying deployment..."
LIVE_JS=$(curl -s https://winbig.bedriften.xyz/index.html | grep -o 'index-[^"]*\.js' | head -1)
echo "Live JS: $LIVE_JS"
echo "Built JS: $JS_NAME"

if [ "$LIVE_JS" = "$JS_NAME" ]; then
    echo "✅ Deployment successful! 🎉"
    echo "🌐 Website: https://winbig.bedriften.xyz"
else
    echo "❌ Deployment verification failed"
    exit 1
fi

cd ..