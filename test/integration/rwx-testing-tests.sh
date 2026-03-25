#!/usr/bin/env bash
set -uo pipefail

exit_code=0
run_result=$(./rwx run --wait --json --file ./rwx-testing/.rwx/trigger-integration-test.yml --init 'grep=@cli' --init ref=main --init lsp=main --init "cli=${COMMIT_SHA}") || exit_code=$?

run_url=$(echo "${run_result}" | jq -r ".RunURL // empty")
result_status=$(echo "${run_result}" | jq -r ".ResultStatus // empty")

if [ -n "$run_url" ]; then
  echo "$run_url" > "$RWX_LINKS/Integration Tests Run"
fi

if [ "$exit_code" -ne 0 ]; then
  echo "rwx run failed with exit code ${exit_code}"
  if [ -n "$run_url" ]; then
    echo "Run URL: ${run_url}"
  fi
  echo "${run_result}"
  exit 1
fi

if [ "$result_status" != "succeeded" ]; then
  echo "Integration tests ${result_status:-unknown}: ${run_url:-no run URL}"
  echo "${run_result}"
  exit 1
fi
