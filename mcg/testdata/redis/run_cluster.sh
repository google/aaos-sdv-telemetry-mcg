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

# Create server configs
for i in 0 1 2; do
 p=$((7000 + i))
 mkdir -p node_$p
 cat << EOF > node_$p/redis.conf
 port $p
 cluster-enabled yes
 cluster-config-file cluster.conf
 cluster-node-timeout 5000
 notify-keyspace-events "KEA"
 appendonly yes
EOF
done;

# Start servers
for i in 0 1 2; do
 (cd node_$((7000 + i)) && redis-server redis.conf &)
done;

# Wait for all servers to be ready
for i in 0 1 2; do
  p=$((7000 + i))
  while ! redis-cli -p "$p" PING > /dev/null 2>&1; do
    sleep 0.1
  done
done

# Create cluster
redis-cli --cluster create 127.0.0.1:7000 127.0.0.1:7001 127.0.0.1:7002 --cluster-yes;

# Run "CLUSTER INFO" every 1 second forever
redis-cli -p 7000 -c -r -1 -i 1 CLUSTER INFO
