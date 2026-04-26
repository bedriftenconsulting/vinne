#!/bin/bash
# Deploy vinne-website to production
# Usage: bash deploy-website.sh

set -e

VM_USER="suraj"
VM_IP="34.42.87.251"
SSH_KEY="/c/Users/Suraj/.ssh/google_compute_engine"
SSH="ssh -i $SSH_KEY -o StrictHostKeyChecking=no $VM_USER@$VM_IP"
REMOTE_DIR="/var/www/winbig"

echo "=== Building website ==="
cd vinne-website
npm run build -- --mode production

echo "=== Uploading files ==="
# Upload index.html
scp -i $SSH_KEY -o StrictHostKeyChecking=no dist/index.html $VM_USER@$VM_IP:/tmp/index.html

# Upload JS (find the current hash)
JS_FILE=$(ls dist/assets/index-*.js)
JS_NAME=$(basename $JS_FILE)
scp -i $SSH_KEY -o StrictHostKeyChecking=no $JS_FILE $VM_USER@$VM_IP:/tmp/$JS_NAME

# Upload CSS
CSS_FILE=$(ls dist/assets/index-*.css)
CSS_NAME=$(basename $CSS_FILE)
scp -i $SSH_KEY -o StrictHostKeyChecking=no $CSS_FILE $VM_USER@$VM_IP:/tmp/$CSS_NAME

echo "=== Deploying on server ==="
$SSH "sudo rm -f $REMOTE_DIR/assets/index-*.js $REMOTE_DIR/assets/index-*.css && \
      sudo cp /tmp/$JS_NAME $REMOTE_DIR/assets/$JS_NAME && \
      sudo cp /tmp/$CSS_NAME $REMOTE_DIR/assets/$CSS_NAME && \
      sudo cp /tmp/index.html $REMOTE_DIR/index.html && \
      sudo nginx -s reload && \
      echo 'Deployed: $JS_NAME'"

echo "=== Verifying ==="
LIVE_JS=$(curl -s https://winbig.bedriften.xyz/index.html | grep -o 'index-[^"]*\.js')
echo "Live JS: $LIVE_JS"
echo "Built JS: $JS_NAME"

if [ "$LIVE_JS" = "$JS_NAME" ]; then
  echo "✅ Deploy successful!"
else
  echo "❌ Mismatch — check server"
fi

cd ..
