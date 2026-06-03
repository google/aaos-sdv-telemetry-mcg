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

FROM redis

EXPOSE 7000-7002/tcp

HEALTHCHECK --start-period=30s --interval=1s CMD redis-cli -p 7000 -c CLUSTER INFO | grep -q 'cluster_state:ok'

ARG DEST_FOLDER=/usr/local/redis_cluster

RUN mkdir -p ${DEST_FOLDER}

COPY run_cluster.sh ${DEST_FOLDER}/run_cluster.sh
WORKDIR ${DEST_FOLDER}

RUN chmod +x run_cluster.sh
CMD ["./run_cluster.sh"]
