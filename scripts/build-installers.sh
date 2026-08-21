#!/usr/bin/env bash
set -e
echo "Building .deb, .rpm, .AppImage, .msi, .dmg via tauri build"
npm --prefix frontend run build
cargo tauri build -- --target all
