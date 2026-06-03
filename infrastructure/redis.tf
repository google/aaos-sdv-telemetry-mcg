/*
 *  Copyright 2025 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

data "google_compute_network" "vpc_network" {
  name = var.network_name
}

resource "google_redis_instance" "cache" {
  project                 = var.project_id
  name                    = "${var.service_name}-redis"
  tier                    = "STANDARD_HA" # Required for AUTH. For dev, you could use BASIC and remove auth.
  memory_size_gb          = 1
  location_id             = var.redis_primary_zone
  alternative_location_id = var.redis_secondary_zone
  authorized_network      = data.google_compute_network.vpc_network.id
  connect_mode            = "DIRECT_PEERING"
  auth_enabled            = true

  # Ensure the Redis API is enabled before creating the instance.
  depends_on = [google_project_service.services]
}
