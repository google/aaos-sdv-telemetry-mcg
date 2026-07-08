#!/bin/bash
# Copyright 2023 Google LLC
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

set -euo pipefail
shopt -s globstar

MCG_REPO="${KOKORO_ARTIFACTS_DIR}/git/mcg"
cd "${MCG_REPO}"

# go/custom-generated-test-logs#converting-bazel-test-logs-to-spongeresultstore-test-logs
# Find all test.xml and test.log files in Bazel's `bazel-testlogs` folder, then:
# 1. Copy them into `${KOKORO_ARTIFACTS_DIR}/bazel-testlogs`: This is important,
#    because `${MCG_REPO}/bazel-testlogs` is a symlink, and Kokoro doesn't follow symlinks
#    when uploading artifacts.
# 2. Rename `test.log` into `sponge_log.log` and `test.xml` into
#    `sponge_log.xml`, which are the filenames Sponge expects.
capture_test_logs() {
  find -L "bazel-testlogs" \( -name "test.log" -o -name "test.xml" \) -exec cp --parents {} "${KOKORO_ARTIFACTS_DIR}" \;
  find "${KOKORO_ARTIFACTS_DIR}/bazel-testlogs" -name "test.log" -exec bash -c 'mv "$1" "${1/%test.log/sponge_log.log}"' bash {} \;
  find "${KOKORO_ARTIFACTS_DIR}/bazel-testlogs" -name "test.xml" -exec bash -c 'mv "$1" "${1/%test.xml/sponge_log.xml}"' bash {} \;
}
# Run capture_test_logs when the script exits
trap capture_test_logs EXIT

# Go wants a cache directory.
export XDG_CACHE_HOME="${TEMP}/.cache"
mkdir "${XDG_CACHE_HOME}"

# Prepend a custom `bin` directory to the `PATH`, so that we can overwrite
# system-installed binaries with our own binaries (e.g., for `bazel`).
CUSTOM_BINARY_DIR="${TEMP}/bin"
mkdir "${CUSTOM_BINARY_DIR}"
export PATH="${CUSTOM_BINARY_DIR}:${PATH}"

# Install Bazelisk as the `bazel` command.
BAZELISK_PATH="${CUSTOM_BINARY_DIR}/bazel"
curl -L https://github.com/bazelbuild/bazelisk/releases/download/v1.25.0/bazelisk-linux-amd64 -o "${BAZELISK_PATH}"
chmod a+x "${BAZELISK_PATH}"
# Check that `bazel` now points to `bazelisk`
if [ "$(which bazel)" != "${BAZELISK_PATH}" ]; then
  echo "Failed to install bazelisk as bazel."
  exit 1
fi

# Set up non-root user. We need this as the Bazel python toolchain does not
# allow to be running as root
# https://github.com/bazelbuild/rules_python/issues/1169
echo "Setting up non-root user for Bazel"
useradd -m -s /bin/bash bazeluser
# Give the user access to the necessary directories.
chmod -R a+rw "${MCG_REPO}"
chmod -R a+rw "${XDG_CACHE_HOME}"

# Grant Bazel tests access to the Docker daemon. The docker daemon acts as a proxy
# to the host's daemon to enable docker-in-docker (DinD). We use the numeric group ID
# (GID) of the mounted socket because the named 'docker' group might not exist inside
# this container image or map to the correct GID from the host.
if [ -e /var/run/docker.sock ]; then
  DOCKER_GID=$(stat -c '%g' /var/run/docker.sock)
  if ! getent group "$DOCKER_GID" > /dev/null 2>&1; then
    groupadd -g "$DOCKER_GID" docker_host
  fi
  DOCKER_GROUP=$(getent group "$DOCKER_GID" | cut -d: -f1)
  usermod -aG "$DOCKER_GROUP" bazeluser
fi

FAILURE=0
# Helper function that runs the provided command, and sets `FAILURE` to 1 if the
# command exits with a non-zero exit code. This allows us to continue running
# checks even if one of them fails.
run_test() {
    echo
    echo "################################################"
    echo Running "$*"
    echo "################################################"
    echo
    if ! "$@"; then
        FAILURE=1
        echo
        echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
        echo "Test failed (see output above)"
        echo "!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!"
        echo
    fi
}

run_test runuser -u bazeluser -- bazel test --test_env=KOKORO=1 --test_output=all //...

run_test runuser -u bazeluser -- bazel run //tools:buildifier -- -lint fix -r .
run_test runuser -u bazeluser -- bazel run //tools:go -- run cmd/gofmt -l -w **/*.go
run_test runuser -u bazeluser -- bazel run //tools:gazelle -- --strict
run_test runuser -u bazeluser -- bazel run //tools:go -- mod tidy
run_test runuser -u bazeluser -- bazel mod tidy

git config --global --add safe.directory "${MCG_REPO}"
run_test git diff --exit-code HEAD

# If any test or check failed, fail the entire build.
if ((FAILURE)); then
    echo "Tests failed (see above)"
    exit 1
fi
