#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/sandbox-helpers.sh"

cd ./rwx-testing
start_sandbox
trap stop_sandbox EXIT

../rwx sandbox exec -- sh -c 'echo "new sandbox file" > new-sandbox-file.txt'
expected_sha=$(echo "new sandbox file" | sha1sum | awk '{print $1}')
actual_sha=$(sha1sum new-sandbox-file.txt | awk '{print $1}')
if [ "$expected_sha" != "$actual_sha" ]; then
  echo "new-sandbox-file.txt content mismatch after sandbox exec (expected $expected_sha, got $actual_sha)"
  exit 1
fi

../rwx sandbox exec -- sh -c 'echo "# Sandbox modification" >> .rwx/sandbox.yml'
modified_sandbox_yml_sha=$(sha1sum .rwx/sandbox.yml | awk '{print $1}')
original_sha=$(git show HEAD:.rwx/sandbox.yml | sha1sum | awk '{print $1}')
if [ "$original_sha" = "$modified_sandbox_yml_sha" ]; then
  echo ".rwx/sandbox.yml was not modified by sandbox exec (sha still $original_sha)"
  exit 1
fi

sandbox_modified_sha=$(../rwx sandbox exec -- sha1sum .rwx/sandbox.yml | awk 'NR==1{print $1}')
if [ "$sandbox_modified_sha" != "$modified_sandbox_yml_sha" ]; then
  echo ".rwx/sandbox.yml local/sandbox mismatch after modification (local: $modified_sandbox_yml_sha, sandbox: $sandbox_modified_sha)"
  exit 1
fi
