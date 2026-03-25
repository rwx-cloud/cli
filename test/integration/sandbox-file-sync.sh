#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/sandbox-helpers.sh"

cd ./rwx-testing
start_sandbox
trap stop_sandbox EXIT

echo "new file" > new-file.txt
echo "# Change to existing file" >> .rwx/sandbox.yml

new_file_sha=$(sha1sum new-file.txt | awk '{print $1}')
changed_file_sha=$(sha1sum .rwx/sandbox.yml | awk '{print $1}')

sandbox_new_file_sha=$(../rwx sandbox exec -- sha1sum new-file.txt | awk 'NR==1{print $1}')
if [ "$new_file_sha" != "$sandbox_new_file_sha" ]; then
  echo "new-file.txt content mismatch in sandbox (local: $new_file_sha, sandbox: $sandbox_new_file_sha)"
  exit 1
fi

changed_file_sha_check=$(../rwx sandbox exec -- sha1sum .rwx/sandbox.yml | awk 'NR==1{print $1}')
if [ "$changed_file_sha" != "$changed_file_sha_check" ]; then
  echo ".rwx/sandbox.yml content mismatch in sandbox (local: $changed_file_sha, sandbox: $changed_file_sha_check)"
  exit 1
fi

post_new_file_sha=$(sha1sum new-file.txt | awk '{print $1}')
if [ "$new_file_sha" != "$post_new_file_sha" ]; then
  echo "new-file.txt was modified during sandbox exec (expected $new_file_sha, got $post_new_file_sha)"
  exit 1
fi

post_sandbox_yml_sha=$(sha1sum .rwx/sandbox.yml | awk '{print $1}')
if [ "$changed_file_sha" != "$post_sandbox_yml_sha" ]; then
  echo ".rwx/sandbox.yml was modified during sandbox exec (expected $changed_file_sha, got $post_sandbox_yml_sha)"
  exit 1
fi
