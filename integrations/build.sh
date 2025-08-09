#!/bin/bash

set -euo pipefail

# Usage: ./build.sh chrome|firefox
# Version is the latest tag in the repo

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
    cp -r firefox/* dist/firefox/
    sed -i -e '/localhost/d' dist/firefox/manifest.json
    sed -i -e "s/\"version\":.*/\"version\": \"$version\",/g" dist/firefox/manifest.json
    # Zip the content of the dist/firefox directory, and not the dist/firefox directory itself
    cd dist/firefox
    zip -r ../firefox.zip * -x "*/.DS_Store"
    cd ../..
    rm -rf dist/firefox
fi

echo "Version $version for $1 is built"
