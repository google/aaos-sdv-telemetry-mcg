<!--
  Copyright 2025 Google LLC

  Licensed under the Apache License, Version 2.0 (the "License");
  you may not use this file except in compliance with the License.
  You may obtain a copy of the License at

      http://www.apache.org/licenses/LICENSE-2.0

  Unless required by applicable law or agreed to in writing, software
  distributed under the License is distributed on an "AS IS" BASIS,
  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
  See the License for the specific language governing permissions and
  limitations under the License.
-->

# Infrastructure for Metrics Config Generator (MCG)

This directory contains the Infrastructure as Code (IaC) to deploy the MCG
service on Google Cloud Platform using Terraform.

## Prerequisites

1.  [Install Terraform](https://learn.hashicorp.com/tutorials/terraform/install-cli)
    (v1.0.0+).
2.  [Install Google Cloud SDK](https://cloud.google.com/sdk/docs/install).
3.  Authenticate with GCP: `gcloud auth application-default login`.
4.  Create a `terraform.tfvars` file in this directory with the following
    content, replacing the placeholder values:

    ```tfvars
    project_id        = "your-gcp-project-id"

    # Optional: Specify a different region and zones for Redis.
    # The zones must be within the specified region.
    # region               = "us-central1"
    # redis_primary_zone   = "us-central1-a"
    # redis_secondary_zone = "us-central1-b"

    # Optional: Specify a different network and VPC connector IP range.
    # The network must exist in the project. The IP range must be a /28 and not overlap with other subnets.
    # network_name                  = "my-custom-network"
    # vpc_connector_ip_cidr_range = "10.10.0.0/28"

    # Optional: To use an existing service account for Cloud Run, uncomment and set the following variable.
    # If you use this, you are responsible for granting it the required permissions
    # (e.g., Secret Manager Secret Accessor, Redis Client, and allowing the Cloud Build SA to impersonate it).
    # run_service_account_email = "my-existing-sa@your-gcp-project-id.iam.gserviceaccount.com"

    # Optional: To disable Terraform from managing Cloud Build IAM permissions, uncomment and set to false.
    # This is useful if permissions are granted at a higher level (e.g., organization or folder).
    # manage_cloudbuild_iam = false
    ```

## External IAM Management

If you manage IAM permissions centrally, you can disable Terraform's IAM
management.

1.  Create a `terraform.tfvars` file and set the following variables. You must
    provide an existing service account for Cloud Run and disable Cloud Build
    IAM management.

    ```tfvars
    project_id                = "your-gcp-project-id"
    run_service_account_email = "your-run-sa@your-gcp-project-id.iam.gserviceaccount.com"
    manage_cloudbuild_iam     = false
    ```

2.  Ensure the necessary permissions are granted to your service accounts using
    your central management tool. You will need to grant:

    -   **Cloud Run SA**: `roles/compute.networkUser` and
        `roles/secretmanager.secretAccessor`.
    -   **Cloud Build SA**: `roles/run.admin`, `roles/artifactregistry.writer`,
        and `roles/iam.serviceAccountUser` (to impersonate the Cloud Run SA).

## Deployment

1.  Initialize Terraform from within this directory: `terraform init`

2.  Review the execution plan: `terraform plan`

3.  Apply the changes to create the resources: `terraform apply`

This will provision the following resources:

-   An **Artifact Registry** repository for your Docker images.
-   A **Memorystore for Redis** instance (Standard HA tier for AUTH support).
-   A **Secret Manager** secret for the Redis password.
-   A **Serverless VPC Access connector** for Cloud Run to connect to Redis.
-   A dedicated **IAM Service Account** for the Cloud Run service with necessary
    permissions.
-   A **Cloud Run** service (initially with a placeholder image).
-   IAM permissions for the Cloud Build service account.

## Redis Configuration

The deployed service can be configured using environment variables. When using
Terraform, these are typically managed automatically, but you might need to know
them for troubleshooting or advanced configuration.

*   `MCG_REDISCACHE_HOSTS`: Format `IP1:port1;IP2:port2...`. Enables Redis for
    persistent storage.
*   `MCG_REDISCACHE_PASS`: Redis Password.
*   `MCG_REDISCACHE_CLUSTER`: If set to `"true"`, forces Redis cluster mode.
    Defaults to cluster mode if multiple hosts are provided in
    `MCG_REDISCACHE_HOSTS`.

## Manual Builds for Local Development

If you do not have a connected repository or cannot configure webhooks, you can
trigger builds manually from your local machine using the Google Cloud SDK. This
is also useful for testing your `cloudbuild.yaml` configuration before setting
up automated triggers.

1.  Navigate to the root directory of this project (the one containing
    `cloudbuild.yaml`).

2.  Run the following command:

    ```bash
    gcloud builds submit . --config=cloudbuild.yaml --substitutions=COMMIT_SHA=$(git rev-parse --short HEAD)
    ```

    -   This command packages your current directory, uploads it to Cloud
        Storage, and starts a build.
    -   The `COMMIT_SHA` substitution is required by the `cloudbuild.yaml` to
        tag the container image. This example uses the short hash of your
        current git commit. You can replace `$(git rev-parse --short HEAD)` with
        any string, like `local-test`.
    -   The other substitution variables (`_SERVICE_NAME`, `_LOCATION`, etc.)
        will use the default values defined in `cloudbuild.yaml`. You can
        override them using the `--substitutions` flag, for example:
        `--substitutions=COMMIT_SHA=test,_SERVICE_NAME=my-test-service`.

## Accessing the Deployed Service

The Terraform configuration in the `infrastructure` directory deploys the MCG
service as a private service on Google Cloud Run. This means that only
authenticated requests from authorized principals (users, service accounts) are
allowed.

### Authorization

To send requests to the service, your user account or a service account needs
the **Cloud Run Invoker** (`roles/run.invoker`) IAM role on the deployed
service. You can grant this permission using the `gcloud` CLI:

Note: Users who have basic IAM roles like Editor (`roles/editor`) or Owner
(`roles/owner`) on the project typically already have permission to invoke Cloud
Run services.

```bash
# Replace mcg-service and europe-west1 with your values if you changed the defaults.
gcloud run services add-iam-policy-binding mcg-service \
  --member="user:your-email@example.com" \
  --role="roles/run.invoker" \
  --region="europe-west1"
```

Replace `your-email@example.com` with the email of the user you want to grant
access to.

### Sending Authenticated Requests

To authenticate your requests, you need to include an OIDC identity token in the
`Authorization` header. You can obtain a token for your user account using the
`gcloud` CLI.

Here is an example of how to call the `generate_metrics_config` API on a
deployed service:

1.  **Get the service URL:**

    ```bash
    # Replace mcg-service and europe-west1 if you used different values.
    SERVICE_URL=$(gcloud run services describe mcg-service --platform managed --region europe-west1 --format 'value(status.url)')
    ```

2.  **Send the request with an identity token:**

    ```bash
    curl -H "Authorization: Bearer $(gcloud auth print-identity-token)" \
      --data-binary @mcg/testdata/eipf_b.json \
      "${SERVICE_URL}/api/v2/generate_metrics_config" \
      -H 'accept: text/x-protobuf' \
      -H 'Content-Type: application/json'
    ```

### Accessing OpenAPI Documentation

To access the interactive OpenAPI documentation in a browser, you must have
**Identity-Aware Proxy (IAP)** enabled for the service.

Ensure your user account has the **IAP-secured Web App User**
(`roles/iap.httpsResourceAccessor`) IAM role on the resource (or project).

Navigate to:

```
https://<SERVICE_URL>/docs
```
