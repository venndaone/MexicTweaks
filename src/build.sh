#!/usr/bin/env sh
# Cross-Build von Linux/macOS aus (benoetigt Go 1.22+)
set -e
cd "$(dirname "$0")"
go mod tidy
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-H windowsgui -s -w" -o ../MexxicTweak100.exe .
echo "Fertig: ../MexxicTweak100.exe"
