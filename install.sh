#!/bin/bash
# Quick installer for Claude2API
# This script downloads and fixes the deploy script

set -e

echo "Downloading Claude2API deployment script..."

# Download and fix line endings in one go
curl -fsSL https://raw.githubusercontent.com/wosa1402/claude2api/main/deploy.sh | tr -d '\r' > /tmp/deploy.sh

chmod +x /tmp/deploy.sh

echo "Deployment script ready!"
echo ""

# Execute the deploy script with any arguments passed
exec /tmp/deploy.sh "$@"
