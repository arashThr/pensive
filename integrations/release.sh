#!/bin/bash

set -euo pipefail

# Usage: ./release.sh chrome|firefox
# Version is the latest tag in the repo

if [ $# -ne 1 ]; then
    echo "Usage: $0 chrome|firefox"
    exit 1
fi

latest_tag=$(git describe --tags --abbrev=0 2>/dev/null || true)
echo "Latest tag detected: $latest_tag"

read -r -p "Have you created and pushed the correct tag for this release? [y/N] " answer
case "$answer" in
    y|yes) ;;
    *) echo "Aborting. Please create/push the tag first."; exit 1 ;;
esac

version=$(git describe --tags --abbrev=0)

if [ "$1" == "chrome" ]; then
    echo "Building Chrome extension"
    mkdir -p dist/chrome
    cp -r chrome/* dist/chrome/
    sed -i -e '/localhost/d' dist/chrome/manifest.json
    sed -i -e "s/\"version\":.*/\"version\": \"$version\",/g" dist/chrome/manifest.json
    zip -r dist/chrome.zip dist/chrome  -x "*/.DS_Store"
    rm -rf dist/chrome
fi

if [ "$1" == "firefox" ]; then
    echo "Building Firefox extension"
    mkdir -p dist/firefox
    cp -r chrome/* dist/firefox/
    cp firefox/manifest.json dist/firefox/
    sed -i -e '/localhost/d' dist/firefox/manifest.json
    sed -i -e "s/\"version\":.*/\"version\": \"$version\",/g" dist/firefox/manifest.json
    # Zip the content of the dist/firefox directory, and not the dist/firefox directory itself
    cd dist/firefox
    zip -r ../firefox.zip * -x "*/.DS_Store"
    cd ../..
    rm -rf dist/firefox
fi

echo "Version $version for $1 is built"
