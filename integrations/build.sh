#!/bin/bash

set -euo pipefail

# Build Chrome and Firefox extensions
# Usage: ./build.sh chrome|firefox

version=$(git describe --tags --abbrev=0)

if [ "$1" == "chrome" ]; then
    mkdir -p dist/chrome
    cp -r chrome/* dist/chrome/
    sed -i -e '/localhost/d' dist/chrome/manifest.json
    sed -i -e "s/\"version\":.*/\"version\": \"$version\",/g" dist/chrome/manifest.json
    zip -r dist/chrome.zip dist/chrome
    rm -rf dist/chrome
fi

if [ "$1" == "firefix" ]; then
    mkdir -p dist/firefox
    cp -r firefox/* dist/firefox/
    sed -i -e '/localhost/d' dist/firefox/manifest.json
    sed -i -e "s/version\":.*/version\": \"$version\"/g" dist/chrome/manifest.json
    zip -r dist/firefox.zip dist/firefox
    rm -rf dist/firefox
fi
