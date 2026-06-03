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

# Serverless VPC Access connector for Cloud Run to reach Redis
resource "google_vpc_access_connector" "connector" {
  project        = var.project_id
  name           = "${var.service_name}-vpc-connector"
  region         = var.region
  ip_cidr_range  = var.vpc_connector_ip_cidr_range
  network        = var.network_name
  min_throughput = 200
  max_throughput = 300
  depends_on     = [google_project_service.services]
}

resource "google_cloud_run_v2_service" "mcg_service" {
  project  = var.project_id
  name     = var.service_name
  location = var.region

  # To allow access from the public internet (required for the gcloud proxy),
  # set ingress to ALL. The service remains secure because Cloud Run requires
  # IAM authentication by default, blocking any unauthenticated requests.
  # Use "INGRESS_TRAFFIC_INTERNAL_ONLY" to block all access from the internet.
  ingress = "INGRESS_TRAFFIC_ALL"

  template {
    service_account = local.service_account_email
    containers {
      # This placeholder image will be replaced by the first Cloud Build run.
      image = "us-docker.pkg.dev/cloudrun/container/hello"
      ports {
        container_port = 8005
      }
      env {
        name  = "MCG_REDISCACHE_HOSTS"
        value = "${google_redis_instance.cache.host}:${google_redis_instance.cache.port}"
      }
      env {
        name = "MCG_REDISCACHE_PASS"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.redis_pass_secret.secret_id
            version = "latest"
          }
        }
      }
    }

    # Needed to connect to Redis via the private network
    vpc_access {
      connector = google_vpc_access_connector.connector.id
      egress    = "ALL_TRAFFIC"
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  depends_on = [
    google_project_service.services,
    google_secret_manager_secret_version.redis_pass_version,
    google_vpc_access_connector.connector
  ]
}
