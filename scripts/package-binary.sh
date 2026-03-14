#!/usr/bin/env bash

set -euo pipefail

if [ "$#" -ne 5 ]; then
  echo "用法: $0 <app_name> <version> <goos> <goarch> <archive_ext>" >&2
  exit 1
fi

APP_NAME="$1"
VERSION="$2"
GOOS="$3"
GOARCH="$4"
ARCHIVE_EXT="$5"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${ROOT_DIR}/dist/${APP_NAME}_${VERSION}_${GOOS}_${GOARCH}"
PACKAGE_DIR="${TARGET_DIR}/package"
ARCHIVE_NAME="${APP_NAME}_${VERSION}_${GOOS}_${GOARCH}"
BIN_NAME="${APP_NAME}"

if [ "${GOOS}" = "windows" ]; then
  BIN_NAME="${APP_NAME}.exe"
fi

rm -rf "${TARGET_DIR}"
mkdir -p "${PACKAGE_DIR}"

(
  cd "${ROOT_DIR}"
  GOOS="${GOOS}" GOARCH="${GOARCH}" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o "${PACKAGE_DIR}/${BIN_NAME}" .

  cp README.md LICENSE config.yaml.example "${PACKAGE_DIR}/"
)

case "${ARCHIVE_EXT}" in
  zip)
    (
      cd "${PACKAGE_DIR}"
      zip -q -r "../${ARCHIVE_NAME}.zip" .
    )
    ARCHIVE_PATH="${TARGET_DIR}/${ARCHIVE_NAME}.zip"
    ;;
  tar.gz)
    tar -C "${PACKAGE_DIR}" -czf "${TARGET_DIR}/${ARCHIVE_NAME}.tar.gz" .
    ARCHIVE_PATH="${TARGET_DIR}/${ARCHIVE_NAME}.tar.gz"
    ;;
  *)
    echo "不支持的压缩格式: ${ARCHIVE_EXT}" >&2
    exit 1
    ;;
esac

rm -rf "${PACKAGE_DIR}"
echo "${ARCHIVE_PATH}"
