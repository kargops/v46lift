#!/usr/bin/env sh
set -eu

PREFIX="${PREFIX:-/usr/local}"
BIN_DIR="${PREFIX}/bin"

install -d "${BIN_DIR}"
install -m 0755 ./bin/v46lift "${BIN_DIR}/v46lift"

echo "Installed ${BIN_DIR}/v46lift"
echo "Install GOST v3 separately or bundle it in your application package."
