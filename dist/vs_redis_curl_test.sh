#!/bin/bash
# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script runs a sequence of curl commands to test the vehicle signal API,
# mirroring the integration test in mcg/signal_versions/signal_api_test.go.

set -euo pipefail

# --- Prerequisites ---
# Ensure gcloud proxy is running on localhost:8080

BASE_URL="http://localhost:8080/api/v2/vs"

if [[ -z "$1" ]]; then
  echo "Usage: $0 <path_to_test_data.json>" >&2
  exit 1
fi
TEST_DATA_FILE="$1"
JQ_BIN=$2

# --- Step 1: Extract the version name from the JSON test file ---
VERSION=$("${JQ_BIN}" -r .version "$TEST_DATA_FILE")
echo "Testing with version: $VERSION"

# --- Helper function for making API calls ---
function call_api() {
  local method="$1"
  local url="$2"
  local expected_status="$3"
  shift 3
  local args=("$@")

  local response_body
  response_body=$(mktemp)
  # The trap will clean up the temp file when the function returns.
  # shellcheck disable=SC2064
  trap "rm -f '${response_body}'" RETURN

  local http_status
  http_status=$(curl --request "${method}" "${url}" \
    --write-out "%{http_code}" \
    --silent \
    --output "${response_body}" \
    "${args[@]}")

  if [[ "${http_status}" -ne "${expected_status}" ]]; then
    echo "ERROR: API call ${method} ${url} failed with status ${http_status} (expected ${expected_status})." >&2
    echo "Response body:" >&2
    cat "${response_body}" >&2
    exit 1
  fi

  # Print response body to stdout for successful calls that should have one.
  cat "${response_body}"
}

# --- Step 2: Add a new vehicle signal version ---
# Corresponds to sendAddRequest() in the Go test.
echo -e "\n--- Sending POST to $BASE_URL/ to create version: $VERSION ---"
call_api "POST" "${BASE_URL}/" 200 \
  -H "Content-Type: application/json" --data "@$TEST_DATA_FILE" > /dev/null
echo "SUCCESS"

# --- Step 3: List all available versions to verify creation ---
# Corresponds to the first sendListRequest() in the Go test.
echo -e "\n--- Sending GET to $BASE_URL/ to list versions ---"
call_api "GET" "${BASE_URL}/" 200 | "${JQ_BIN}"

# --- Step 4: Delete the vehicle signal version ---
# Corresponds to sendDelRequest() in the Go test.
echo -e "\n--- Sending DELETE to $BASE_URL/$VERSION to remove version ---"
call_api "DELETE" "${BASE_URL}/${VERSION}" 200 > /dev/null
echo "SUCCESS"

# --- Step 5: List versions again to confirm deletion ---
# Corresponds to the second sendListRequest() in the Go test.
echo -e "\n--- Sending GET to $BASE_URL/ to confirm deletion ---"
call_api "GET" "${BASE_URL}/" 200 | "${JQ_BIN}"
echo -e "\nAPI test completed successfully."
