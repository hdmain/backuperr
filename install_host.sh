#!/usr/bin/env bash

set -euo pipefail

URL="https://github.com/hdmain/backuperr/releases/download/v1.0.0/host"

FILE="backuperr_host"

echo "Downloading Backuperr from: $URL ..."
curl -L -o "$FILE" "$URL"

echo "Setting executable permissions..."
chmod +x "$FILE"

echo "Installation completed!"
echo "The file is in the current directory and can be run with: ./backuperr_host"