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

variable "project_id" {
  description = "The GCP project ID."
  type        = string
}

variable "region" {
  description = "The GCP region for resources."
  type        = string
  default     = "europe-west1"
}

variable "redis_primary_zone" {
  description = "The primary zone for the STANDARD_HA Redis instance (e.g., 'europe-west1-b'). Must be in the same region as `var.region`."
  type        = string
  default     = "europe-west1-b"
}

variable "redis_secondary_zone" {
  description = "The secondary zone for the STANDARD_HA Redis instance (e.g., 'europe-west1-c'). Must be in the same region as `var.region`."
  type        = string
  default     = "europe-west1-c"
}

variable "service_name" {
  description = "The name of the Cloud Run service."
  type        = string
  default     = "mcg-service"
}

variable "artifact_registry_repo_name" {
  description = "The name of the Artifact Registry repository."
  type        = string
  default     = "mcg-repo"
}

variable "network_name" {
  description = "The name of the VPC network to use for resources like Redis and the VPC connector."
  type        = string
  default     = "default"
}

variable "vpc_connector_ip_cidr_range" {
  description = "The IP CIDR range for the Serverless VPC Access connector. Must be a /28 subnet that is not in use."
  type        = string
  default     = "10.9.0.0/28"
}

variable "run_service_account_email" {
  description = "Optional. The email of an existing IAM Service Account for the Cloud Run service to use. If not provided, a new one will be created and configured with the necessary permissions."
  type        = string
  default     = null

  validation {
    condition     = var.run_service_account_email == null || can(regex("^.+@.+\\.gserviceaccount\\.com$", var.run_service_account_email))
    error_message = "Value must be a valid service account email (e.g., 'name@project-id.iam.gserviceaccount.com') or null."
  }
}

variable "manage_cloudbuild_iam" {
  description = "If true, Terraform will manage IAM permissions for the Cloud Build service account (e.g., roles for deploying to Cloud Run and pushing to Artifact Registry). Set to false if these permissions are managed externally."
  type        = bool
  default     = true
}
