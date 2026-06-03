#!/bin/bash
# Copyright 2024 Google LLC
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

bazel build //dist:image_mcg_tarball
docker image load --input bazel-bin/dist/image_mcg_tarball/tarball.tar | tee /tmp/mcg_docker_image_id
sed -i 's/Loaded image ID: //' /tmp/mcg_docker_image_id
docker image tag "$(cat /tmp/mcg_docker_image_id)" android-sdv-telemetry-mcg:dev
docker run android-sdv-telemetry-mcg:dev
