#!/usr/bin/env bash
set -euo pipefail

cd ./rwx-testing

sandbox_result=$(../rwx sandbox start .rwx/sandbox.yml --json --init 'grep=@cli' --init ref=main --init "cli=${COMMIT_SHA}" --wait)
echo "${sandbox_result}"

sandbox_url=$(echo "${sandbox_result}" | jq -r ".RunURL")
if [ -n "$sandbox_url" ] && [ "$sandbox_url" != "null" ]; then
  echo "$sandbox_url" > "$RWX_LINKS/Sandbox Run"
fi

echo "new file" > new-file.txt
echo "# Change to existing file" >> .rwx/sandbox.yml

new_file_sha=$(sha1sum new-file.txt | awk '{print $1}')
changed_file_sha=$(sha1sum .rwx/sandbox.yml | awk '{print $1}')

sandbox_new_file_sha=$(../rwx sandbox exec -- sha1sum new-file.txt | awk 'NR==1{print $1}')
if [ "$new_file_sha" != "$sandbox_new_file_sha" ]; then
  echo "new-file.txt content mismatch in sandbox (local: $new_file_sha, sandbox: $sandbox_new_file_sha)"
  ../rwx sandbox stop
  exit 1
fi

changed_file_sha_check=$(../rwx sandbox exec -- sha1sum .rwx/sandbox.yml | awk 'NR==1{print $1}')
if [ "$changed_file_sha" != "$changed_file_sha_check" ]; then
  echo ".rwx/sandbox.yml content mismatch in sandbox (local: $changed_file_sha, sandbox: $changed_file_sha_check)"
  ../rwx sandbox stop
  exit 1
fi

post_new_file_sha=$(sha1sum new-file.txt | awk '{print $1}')
if [ "$new_file_sha" != "$post_new_file_sha" ]; then
  echo "new-file.txt was modified during sandbox exec (expected $new_file_sha, got $post_new_file_sha)"
  ../rwx sandbox stop
  exit 1
fi

post_sandbox_yml_sha=$(sha1sum .rwx/sandbox.yml | awk '{print $1}')
if [ "$changed_file_sha" != "$post_sandbox_yml_sha" ]; then
  echo ".rwx/sandbox.yml was modified during sandbox exec (expected $changed_file_sha, got $post_sandbox_yml_sha)"
  ../rwx sandbox stop
  exit 1
fi

../rwx sandbox exec -- sh -c 'echo "new sandbox file" > new-sandbox-file.txt'
expected_sha=$(echo "new sandbox file" | sha1sum | awk '{print $1}')
actual_sha=$(sha1sum new-sandbox-file.txt | awk '{print $1}')
if [ "$expected_sha" != "$actual_sha" ]; then
  echo "new-sandbox-file.txt content mismatch after sandbox exec (expected $expected_sha, got $actual_sha)"
  ../rwx sandbox stop
  exit 1
fi

../rwx sandbox exec -- sh -c 'echo "# Sandbox modification" >> .rwx/sandbox.yml'
modified_sandbox_yml_sha=$(sha1sum .rwx/sandbox.yml | awk '{print $1}')
if [ "$changed_file_sha" = "$modified_sandbox_yml_sha" ]; then
  echo ".rwx/sandbox.yml was not modified by sandbox exec (sha still $changed_file_sha)"
  ../rwx sandbox stop
  exit 1
fi
# Verify that re-syncing the modified file to the sandbox produces a consistent result
sandbox_modified_sha=$(../rwx sandbox exec -- sha1sum .rwx/sandbox.yml | awk 'NR==1{print $1}')
if [ "$sandbox_modified_sha" != "$modified_sandbox_yml_sha" ]; then
  echo ".rwx/sandbox.yml local/sandbox mismatch after modification (local: $modified_sandbox_yml_sha, sandbox: $sandbox_modified_sha)"
  ../rwx sandbox stop
  exit 1
fi

# Verify shell quoting: "bash -c" with a multi-word argument
bash_c_output=$(../rwx sandbox exec --no-sync -- bash -c "echo hello world" | awk 'NR==1')
if [ "$bash_c_output" != "hello world" ]; then
  echo "bash -c shell quoting failed (expected 'hello world', got '$bash_c_output')"
  ../rwx sandbox stop
  exit 1
fi

# Test: uncommitted changes survive when branch has unpushed commits
git commit --allow-empty -m "unpushed local commit"
echo "uncommitted edit" > uncommitted-test.txt
uncommitted_sha=$(sha1sum uncommitted-test.txt | awk '{print $1}')

../rwx sandbox exec -- echo "exercising sandbox with unpushed commits"

post_exec_sha=$(sha1sum uncommitted-test.txt | awk '{print $1}')
if [ "$uncommitted_sha" != "$post_exec_sha" ]; then
  echo "uncommitted-test.txt was lost during sandbox exec with unpushed commits (expected $uncommitted_sha, got $post_exec_sha)"
  ../rwx sandbox stop
  exit 1
fi

# Test: reverting local changes doesn't leak stale sandbox state back.
# The bug only triggers when the local patch is completely empty, so we must
# start from a fully clean working tree.
git checkout .
git clean -fd

echo "revert-test content" > revert-test.txt
../rwx sandbox exec -- cat revert-test.txt > /dev/null
rm -f revert-test.txt

../rwx sandbox exec -- echo "exec after local revert"

if [ -f revert-test.txt ]; then
  echo "revert-test.txt was pulled back from sandbox after being reverted locally"
  ../rwx sandbox stop
  exit 1
fi

../rwx sandbox stop
