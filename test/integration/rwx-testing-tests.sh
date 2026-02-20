#!/usr/bin/env bash
set -euo pipefail

run_result=$(./rwx run --wait --json --file ./rwx-testing/.rwx/trigger-integration-test.yml --init 'grep=@cli' --init ref=main --init lsp=main --init "cli=${COMMIT_SHA}")
echo "${run_result}"

run_url=$(echo "${run_result}" | jq -r ".RunURL")
if [ -n "$run_url" ] && [ "$run_url" != "null" ]; then
  echo "$run_url" > "$RWX_LINKS/Integration Tests Run"
fi

result_status=$(echo "${run_result}" | jq -r ".ResultStatus")

if [ "$result_status" != "succeeded" ]; then
  exit 1
fi
