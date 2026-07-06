#!/bin/bash
set -euo pipefail

APP="DLP-Go"
OUTDIR="release"

echo "=== Validando código ==="
go vet ./...

echo "=== Limpiando releases anteriores ==="
rm -rf "$OUTDIR"
mkdir -p "$OUTDIR"

echo "=== Compilando Linux amd64 ==="
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o "$APP" .
mkdir -p "$OUTDIR/$APP-linux"
mv "$APP" "$OUTDIR/$APP-linux/"
cd "$OUTDIR" && zip -r "${APP}-linux-amd64.zip" "$APP-linux/" && cd ..
rm -rf "$OUTDIR/$APP-linux"

echo "=== Compilando Windows amd64 ==="
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "$APP.exe" .
mkdir -p "$OUTDIR/$APP-windows"
mv "$APP.exe" "$OUTDIR/$APP-windows/"
cd "$OUTDIR" && zip -r "${APP}-windows-amd64.zip" "$APP-windows/" && cd ..
rm -rf "$OUTDIR/$APP-windows"

echo ""
echo "=== Release listo ==="
ls -lh "$OUTDIR/"