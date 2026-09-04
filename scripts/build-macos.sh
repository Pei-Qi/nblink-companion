#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
MACOS_DIR="$DIST_DIR/macos"
ASSET_DIR="$DIST_DIR/assets"
EXECUTABLE_NAME="nblink-companion"
APP_NAME="Nblink Companion.app"
ICON_NAME="NblinkCompanion.icns"
VERSION="${VERSION:-0.3.0}"
BUILD_NUMBER="${BUILD_NUMBER:-1}"
export GOCACHE="${GOCACHE:-$DIST_DIR/.gocache}"

mkdir -p "$MACOS_DIR" "$ASSET_DIR"

(
  cd "$ROOT_DIR/frontend"
  npm ci
  npm run build
)

go run "$ROOT_DIR/cmd/iconbuilder" \
  -svg "$ROOT_DIR/assets/app-icon.svg" \
  -png "$ASSET_DIR/NblinkCompanion-1024.png" \
  -icns "$ASSET_DIR/$ICON_NAME"

go run "$ROOT_DIR/cmd/iconbuilder" \
  -svg "$ROOT_DIR/assets/tray-icon.svg" \
  -png "$ROOT_DIR/assets/tray-icon.png" \
  -png-size 64

for arch in arm64 amd64; do
  ARCH_DIR="$MACOS_DIR/$arch"
  APP_DIR="$ARCH_DIR/$APP_NAME"
  ZIP_PATH="$DIST_DIR/Nblink-Companion-$VERSION-macos-$arch.zip"
  rm -rf "$APP_DIR" "$ZIP_PATH"
  mkdir -p "$APP_DIR/Contents/MacOS" "$APP_DIR/Contents/Resources"

  clang_arch="$arch"
  if [[ "$arch" == "amd64" ]]; then
    clang_arch="x86_64"
  fi
  CGO_ENABLED=1 GOOS=darwin GOARCH="$arch" CC=clang \
    CGO_CFLAGS="-arch $clang_arch -mmacosx-version-min=12.0" \
    CGO_LDFLAGS="-arch $clang_arch -mmacosx-version-min=12.0" \
    go build -trimpath -tags production -ldflags="-s -w" \
      -o "$APP_DIR/Contents/MacOS/$EXECUTABLE_NAME" "$ROOT_DIR/cmd/nblink-companion"

  cp "$ASSET_DIR/$ICON_NAME" "$APP_DIR/Contents/Resources/$ICON_NAME"
  cat >"$APP_DIR/Contents/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "https://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleDevelopmentRegion</key>
  <string>zh_CN</string>
  <key>CFBundleDisplayName</key>
  <string>节点小宝固定端口伴侣</string>
  <key>CFBundleExecutable</key>
  <string>$EXECUTABLE_NAME</string>
  <key>CFBundleIconFile</key>
  <string>$ICON_NAME</string>
  <key>CFBundleIdentifier</key>
  <string>com.local.nblink-companion</string>
  <key>CFBundleInfoDictionaryVersion</key>
  <string>6.0</string>
  <key>CFBundleName</key>
  <string>Nblink Companion</string>
  <key>CFBundlePackageType</key>
  <string>APPL</string>
  <key>CFBundleShortVersionString</key>
  <string>$VERSION</string>
  <key>CFBundleVersion</key>
  <string>$BUILD_NUMBER</string>
  <key>LSApplicationCategoryType</key>
  <string>public.app-category.utilities</string>
  <key>LSMinimumSystemVersion</key>
  <string>12.0</string>
  <key>LSUIElement</key>
  <true/>
  <key>NSHighResolutionCapable</key>
  <true/>
</dict>
</plist>
PLIST

  plutil -lint "$APP_DIR/Contents/Info.plist"
  codesign --force --deep --sign - "$APP_DIR"
  file "$APP_DIR/Contents/MacOS/$EXECUTABLE_NAME"
  codesign --verify --deep --strict "$APP_DIR"
  ditto -c -k --sequesterRsrc --keepParent "$APP_DIR" "$ZIP_PATH"
  printf 'Created %s\n' "$APP_DIR"
  printf 'Created %s\n' "$ZIP_PATH"
done
