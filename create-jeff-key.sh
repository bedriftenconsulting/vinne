#!/bin/bash
set -e

echo "=== Create jeff user ==="
sudo useradd -m -s /bin/bash jeff 2>/dev/null || echo "user jeff already exists"

echo ""
echo "=== Generate SSH key for jeff ==="
sudo -u jeff bash -c "
  mkdir -p ~/.ssh
  chmod 700 ~/.ssh
  ssh-keygen -t ed25519 -f ~/.ssh/jeff_key -N '' -C 'jeff@winbig'
  cat ~/.ssh/jeff_key.pub >> ~/.ssh/authorized_keys
  chmod 600 ~/.ssh/authorized_keys
"

echo ""
echo "=== Give jeff docker access (read only via sudo) ==="
echo 'jeff ALL=(ALL) NOPASSWD: /usr/bin/docker exec * psql *' | sudo tee /etc/sudoers.d/jeff-docker

echo ""
echo "=== Jeff private key (send this to Jeff) ==="
sudo cat /home/jeff/.ssh/jeff_key
