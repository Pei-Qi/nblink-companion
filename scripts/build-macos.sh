#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT_DIR/dist"
MACOS_DIR="$DIST_DIR/macos"
ASSET_DIR="$DIST_DIR/assets"
EXECUTABLE_NAME="nblink-companion"
APP_NAME="Nblink Companion.app"
ICON_NAME="NblinkCompanion.icns"
VERSION="${VERSION:-$(cd "$ROOT_DIR" && node -p "require('./frontend/package.json').version")}"
BUILD_NUMBER="${BUILD_NUMBER:-1}"
ARCHS="${ARCHS:-arm64 amd64}"
SIGN_IDENTITY="${MACOS_SIGN_IDENTITY:--}"
REQUIRE_NOTARIZATION="${REQUIRE_MACOS_NOTARIZATION:-0}"
export GOCACHE="${GOCACHE:-$DIST_DIR/.gocache}"

notarization_enabled=0
if [[ -n "${MACOS_NOTARY_PROFILE:-}" ]]; then
  notarization_enabled=1
elif [[ -n "${MACOS_NOTARY_APPLE_ID:-}" && -n "${MACOS_NOTARY_TEAM_ID:-}" && -n "${MACOS_NOTARY_PASSWORD:-}" ]]; then
  notarization_enabled=1
fi

if [[ "$REQUIRE_NOTARIZATION" == "1" && ( "$SIGN_IDENTITY" == "-" || "$notarization_enabled" != "1" ) ]]; then
  printf 'A release build requires MACOS_SIGN_IDENTITY and notarization credentials.\n' >&2
  exit 1
fi

mkdir -p "$MACOS_DIR" "$ASSET_DIR"

if [[ "${SKIP_FRONTEND_BUILD:-}" != "1" ]]; then
  (
    cd "$ROOT_DIR/frontend"
    npm ci
    npm run build
  )
fi

go run "$ROOT_DIR/cmd/iconbuilder" \
  -svg "$ROOT_DIR/assets/app-icon.svg" \
  -png "$ASSET_DIR/NblinkCompanion-1024.png" \
  -icns "$ASSET_DIR/$ICON_NAME"

go run "$ROOT_DIR/cmd/iconbuilder" \
  -svg "$ROOT_DIR/assets/tray-icon.svg" \
  -png "$ROOT_DIR/assets/tray-icon.png" \
  -png-size 64

read -r -a arch_list <<<"$ARCHS"
for arch in "${arch_list[@]}"; do
  case "$arch" in
    arm64 | amd64) ;;
    *)
      printf 'Unsupported macOS architecture: %s\n' "$arch" >&2
      exit 1
      ;;
  esac

  ARCH_DIR="$MACOS_DIR/$arch"
  APP_DIR="$ARCH_DIR/$APP_NAME"
  ZIP_PATH="$DIST_DIR/Nblink-Companion-$VERSION-macos-$arch.zip"
  CHECKSUM_PATH="$ZIP_PATH.sha256"
  rm -rf "$APP_DIR" "$ZIP_PATH" "$CHECKSUM_PATH"
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
  codesign_args=(--force --deep --sign "$SIGN_IDENTITY")
  if [[ "$SIGN_IDENTITY" != "-" ]]; then
    codesign_args+=(--options runtime --timestamp)
  fi
  codesign "${codesign_args[@]}" "$APP_DIR"
  file "$APP_DIR/Contents/MacOS/$EXECUTABLE_NAME"
  codesign --verify --deep --strict "$APP_DIR"
  ditto -c -k --sequesterRsrc --keepParent "$APP_DIR" "$ZIP_PATH"

  if [[ "$notarization_enabled" == "1" ]]; then
    if [[ "$SIGN_IDENTITY" == "-" ]]; then
      printf 'Notarization requires a Developer ID Application signing identity.\n' >&2
      exit 1
    fi
    if [[ -n "${MACOS_NOTARY_PROFILE:-}" ]]; then
      xcrun notarytool submit "$ZIP_PATH" --keychain-profile "$MACOS_NOTARY_PROFILE" --wait
    else
      xcrun notarytool submit "$ZIP_PATH" \
        --apple-id "$MACOS_NOTARY_APPLE_ID" \
        --team-id "$MACOS_NOTARY_TEAM_ID" \
        --password "$MACOS_NOTARY_PASSWORD" \
        --wait
    fi
    xcrun stapler staple "$APP_DIR"
    xcrun stapler validate "$APP_DIR"
    spctl --assess --type execute --verbose=2 "$APP_DIR"
    rm -f "$ZIP_PATH"
    ditto -c -k --sequesterRsrc --keepParent "$APP_DIR" "$ZIP_PATH"
  fi

  checksum="$(openssl dgst -sha256 "$ZIP_PATH" | awk '{print $NF}')"
  printf '%s  %s\n' "$checksum" "$(basename "$ZIP_PATH")" >"$CHECKSUM_PATH"
  printf 'Created %s\n' "$APP_DIR"
  printf 'Created %s\n' "$ZIP_PATH"
  printf 'Created %s\n' "$CHECKSUM_PATH"
done
