#!/usr/bin/env sh
set -eu

cd "$(dirname "$0")/.."

npm --prefix web/miniapp ci
npm --prefix web/miniapp run build
npm --prefix web/admin ci
npm --prefix web/admin run build

mkdir -p backend/public/app backend/public/admin
cp -R web/miniapp/dist/. backend/public/app/
cp -R web/admin/dist/. backend/public/admin/
