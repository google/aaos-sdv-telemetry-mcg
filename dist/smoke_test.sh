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

set -euo pipefail

## Pre-release smoke test to verify basic functionality of the Docker container.
## Run manually before approving a release.

# ensure password asked
echo Starting smoke test...

docker image load --input dist/image_mcg_tarball/tarball.tar | tee /tmp/mcg_docker_image_id
sed -i 's/Loaded image ID: //' /tmp/mcg_docker_image_id
docker image tag "$(cat /tmp/mcg_docker_image_id)" android-sdv-telemetry-mcg:dev

container_id="$(docker run --expose 8005 --detach android-sdv-telemetry-mcg:dev)"
kill_container() {
    echo "Cleaning up container..."
    docker kill "$container_id"
}
trap kill_container EXIT

ip_address="$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' "$container_id")"
addr="http://$ip_address:8005"

curl -sSf "$addr"/health

curl -sSf -o /tmp/mcg_smoke_test_v1.pb -X POST "$addr/api/v1/generate_metrics_config" -H 'content-type:application/json' -d '{"triggers":[{"name": "p", "periodic": {"period_ms": 1}}],"start_trigger_name":"p"}'
protoc --decode_raw < /tmp/mcg_smoke_test_v1.pb

validate_response="$(curl -sSf -o /tmp/mcg_smoke_test_v1.pb -X POST "$addr/api/v1/validate_metrics_config" -H 'content-type:text/x-protobuf' -H 'accept:application/json' -w "%{http_code}" -d 'uuid: "dc3f523e-8b3c-4d86-b702-17fec8b5f6c2" version: 3758096386')"
echo "$validate_response"
[ "$validate_response" -eq 200 ]

curl -sf -X GET "$addr/api/v1/version"

# V2 Tests
curl -sSf -o /tmp/mcg_smoke_test_v2.pb -X POST "$addr/api/v2/generate_metrics_config" -H 'content-type:application/json' -d '{"triggers":[{"name": "p", "periodic": {"period_ms": 1}}],"start_trigger_name":"p"}'
protoc --decode_raw < /tmp/mcg_smoke_test_v2.pb

validate_response_v2="$(curl -sSf -o /tmp/mcg_smoke_test_v2.pb -X POST "$addr/api/v2/validate_metrics_config" -H 'content-type:text/x-protobuf' -H 'accept:application/json' -w "%{http_code}" -d 'uuid: "dc3f523e-8b3c-4d86-b702-17fec8b5f6c2" version: 3758096386')"
echo "$validate_response_v2"
[ "$validate_response_v2" -eq 200 ]

curl -sf -X GET "$addr/api/v2/version"

echo $'\n'PASS
