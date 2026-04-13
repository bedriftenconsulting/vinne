#!/bin/bash
# Script to test Docker build locally

echo "Building Docker image locally..."

# Build the image
docker build -t rand-admin-web:local .

if [ $? -eq 0 ]; then
    echo "✅ Docker build successful!"
    echo ""
    echo "To run the container locally:"
    echo "  docker run -p 8080:80 rand-admin-web:local"
    echo ""
    echo "Then access the app at http://localhost:8080"
else
    echo "❌ Docker build failed!"
    exit 1
fi