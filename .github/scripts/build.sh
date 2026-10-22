#!/usr/bin/env bash
# Builds the Windows release files into dist/: wtfsync.exe (CLI), wtfsync-tray.exe (tray app),
# wtfsync-setup.exe (installer, needs makensis) and checksums.txt.
# usage: build.sh VERSION   (e.g. v0.5.0, or dev)
set -euo pipefail

version=${1:?usage: build.sh VERSION}
ldflags="-s -w -X main.version=$version"
rm -rf dist && mkdir dist
go run ./internal/icons/syso cmd/wtfsync cmd/wtfsync-tray # the exes' icon
export CGO_ENABLED=0 GOOS=windows GOARCH=amd64
go build -trimpath -ldflags "$ldflags" -o dist/wtfsync.exe ./cmd/wtfsync
go build -trimpath -ldflags "$ldflags -H windowsgui" -o dist/wtfsync-tray.exe ./cmd/wtfsync-tray
makensis -NOCD -V2 -WX -DVERSION="$version" -DBINDIR=dist -DOUTFILE=dist/wtfsync-setup.exe installer/wtfsync.nsi
(cd dist && sha256sum wtfsync.exe wtfsync-tray.exe wtfsync-setup.exe > checksums.txt)
