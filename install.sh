#!/usr/bin/env bash


set -e


REPO_OWNER="prasaisushant" 
REPO_NAME="tshoot"
BINARY_NAME="tshoot-linux-amd64"
TARGET_DIR="/usr/local/bin"

echo " Fetching latest release information for ${REPO_NAME}..."

# 1. Use the GitHub API to dynamically find the latest version tag (e.g., v1.0.2)
LATEST_TAG=$(curl -s "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_TAG" ]; then
    echo "Error: Could not determine the latest release version."
    exit 1
fi

echo "Downloading version ${LATEST_TAG}..."

# 2. Download the compiled static binary from your automated release page
DOWNLOAD_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${LATEST_TAG}/${BINARY_NAME}"
curl -L -o tshoot "${DOWNLOAD_URL}"

# 3. Secure the file permissions to make it an executable application
echo " Setting executable permissions..."
chmod +x tshoot

# 4. Move it into the system's global $PATH directory
echo "Moving tshoot into ${TARGET_DIR} (requires sudo)..."
sudo mv tshoot "${TARGET_DIR}/tshoot"

echo "Success! tshoot ${LATEST_TAG} has been installed globally."
echo "Try running it now by typing: tshoot"