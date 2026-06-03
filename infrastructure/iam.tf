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

locals {
  # If an existing service account is provided, we won't create a new one or its associated IAM bindings.
  create_new_service_account = var.run_service_account_email == null
  # Use the provided service account email, or the email of the one we create.
  service_account_email = local.create_new_service_account ? google_service_account.run_sa[0].email : var.run_service_account_email
}

data "google_project" "project" {
  project_id = var.project_id
}

# Service account for the Cloud Run service to run as.
# This is only created if an existing service account email is not provided.
resource "google_service_account" "run_sa" {
  count        = local.create_new_service_account ? 1 : 0
  project      = var.project_id
  account_id   = "${var.service_name}-run"
  display_name = "Service Account for MCG Cloud Run service"
}

# Grant Secret Manager accessor role to the Cloud Run service account.
# This is only created if a new service account is created.
resource "google_secret_manager_secret_iam_member" "secret_accessor" {
  count     = local.create_new_service_account ? 1 : 0
  project   = google_secret_manager_secret.redis_pass_secret.project
  secret_id = google_secret_manager_secret.redis_pass_secret.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.run_sa[0].email}"
}

# Grant Redis client role to the Cloud Run service account.
# This is only created if a new service account is created.
resource "google_project_iam_member" "redis_client" {
  count   = local.create_new_service_account ? 1 : 0
  project = var.project_id
  role    = "roles/compute.networkUser"
  member  = "serviceAccount:${google_service_account.run_sa[0].email}"
}

# --- Cloud Build IAM Resources (Optional) ---

# Dedicated service account for the Cloud Build pipeline.
resource "google_service_account" "build_sa" {
  count        = var.manage_cloudbuild_iam ? 1 : 0
  project      = var.project_id
  account_id   = "${var.service_name}-build"
  display_name = "Service Account for MCG Cloud Build pipeline"
}

# Grant the default Cloud Build service account permission to deploy to Cloud Run.
resource "google_project_iam_member" "cloudbuild_run_admin" {
  count   = var.manage_cloudbuild_iam ? 1 : 0
  project = var.project_id
  role    = "roles/run.admin"
  member  = "serviceAccount:${google_service_account.build_sa[0].email}"
}

# Grant the Cloud Build service account permission to push to Artifact Registry.
resource "google_project_iam_member" "cloudbuild_ar_writer" {
  count   = var.manage_cloudbuild_iam ? 1 : 0
  project = var.project_id
  role    = "roles/artifactregistry.writer"
  member  = "serviceAccount:${google_service_account.build_sa[0].email}"
}

# Allow the Cloud Build service account to act as the Cloud Run service account during deployment.
# This is only created if we are managing Cloud Build IAM AND a new service account is being created.
resource "google_service_account_iam_member" "cloudbuild_actas_run_sa" {
  count              = var.manage_cloudbuild_iam && local.create_new_service_account ? 1 : 0
  service_account_id = google_service_account.run_sa[0].name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.build_sa[0].email}"
}
