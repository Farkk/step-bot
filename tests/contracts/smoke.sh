#!/usr/bin/env sh
set -eu

base_url=${1:-http://localhost:8081}
base_url=${base_url%/}

assert_status() {
  expected=$1
  method=$2
  path=$3
  actual=$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' --max-time 10 --request "$method" "$base_url$path")
  if [ "$actual" != "$expected" ]; then
    printf 'FAIL %s %s: expected %s, got %s\n' "$method" "$path" "$expected" "$actual" >&2
    exit 1
  fi
  printf 'OK %s %s: %s\n' "$method" "$path" "$actual"
}

assert_status 200 GET /health/live
assert_status 200 GET /health/ready
assert_status 200 GET /app/
assert_status 200 GET /admin/
assert_status 401 GET /api/v1/auth/session
assert_status 401 GET /api/v1/worker/tasks
assert_status 401 POST /integrations/max/webhook
