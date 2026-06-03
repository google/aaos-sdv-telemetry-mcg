<!--
  Copyright 2024 Google LLC

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

# Metrics Configuration Generator

The Metrics Configuration Generator (MCG) is a compilation and validation
service for SDV telemetry configurations. It functions as a backend for
configuration tooling, exposing a RESTful API that transforms high-level JSON
specifications into the binary `google.sdv.telemetry.MetricsConfig` protobuf
format required by the on-vehicle telemetry client.

## Repository Structure

*   `mcg/`: Source code for the Metrics Configuration Generator service.
*   `third_party/aosp/sdv/telemetry/metrics_configuration`: Protocol buffer
    definitions.
    *   `metrics_configuration/`: Contains `metrics_configuration.proto` and
        `expression.proto`. \
        **Note:** These definitions originate from the Android Open Source
        Project (AOSP).
*   `infrastructure/`: Terraform and cloud build configurations for deployment.
*   `main.go`: Service entry point.

## Getting Started

### Prerequisites

To build and run MCG, you need **Bazel**.

Follow the [official Bazel installation guide](https://bazel.build/install).

### Run locally

To run the MCG server from its source directory for local development and
testing, use Bazel. Enable in-memory caching by setting `MCG_LOCALCACHE=true`
when running locally.

**Environment Variables for Local Execution:**

*   `MCG_LOCALCACHE`: If set to `true`, local in-memory cache is enabled.
*   `MCG_LOCALCACHE_CAP`: Local Cache Capacity (not set or zero is treated as no
    cache size limit).

```bash
# Build and run on default port 8005
MCG_LOCALCACHE=true bazel run //:mcg

# Or, to change the port, use:
MCG_LOCALCACHE=true bazel run //:mcg -- --listen :9000
```

### Running Tests

To run all tests in the repository:

```bash
bazel test //...
```

## Usage

### 1. Register Vehicle Signals

MCG requires a protobuf `FileDescriptorSet` containing your signal definitions
(VSIDL) to perform validation and type inference.

**Generate FileDescriptorSet:**

```bash
protoc --include_imports --descriptor_set_out=vehicle_signals.pb path/to/your/protos/*.proto
```

**Upload Catalog:**

This registers the signals under a specific version (e.g., `v1.0`).

```bash
# Set the service URL (default for local execution)
SERVICE_URL="http://localhost:8005"

# Encode to Base64 and upload
VEHICLE_SIGNALS=$(base64 -w 0 vehicle_signals.pb)
jq -n --arg v "v1.0" --arg d "$VEHICLE_SIGNALS" '{version: $v, vehicle_signals: $d}' \
  | curl -H "Content-Type: application/json" --data-binary @- "$SERVICE_URL/api/v2/vs/"
```

### 2. Generate Metrics Configuration

Submit a JSON configuration to compile it into the binary protobuf format.
Ensure your configuration references the uploaded catalog version (e.g.,
`"vs_version": "v1.0"`).

**Debug with textproto output:**

Requesting `text/x-protobuf` returns a human-readable format, useful for
inspection.

```bash
# Set the service URL (default for local execution)
SERVICE_URL="http://localhost:8005"

curl -H "Content-Type: application/json" \
  -H "Accept: text/x-protobuf" \
  --data-binary @metrics_config.json \
  "$SERVICE_URL/api/v2/generate_metrics_config"
```

**Generate binary protobuf (for device use):**

For deployment to the telemetry service (for testing or in production), you must
use the binary `application/x-protobuf` format.

> **Note:** If the request fails (e.g., validation errors), the API returns a
> JSON error response. Be careful when redirecting output to a file, as you
> might inadvertently save the JSON error details into your `.pb` file.

```bash
# Set the service URL (default for local execution)
SERVICE_URL="http://localhost:8005"

curl -H "Content-Type: application/json" \
  -H "Accept: application/x-protobuf" \
  --data-binary @metrics_config.json \
  "$SERVICE_URL/api/v2/generate_metrics_config" > metrics_config.pb
```

## Deployment

The repository includes Terraform and Cloud Build configurations for deploying
to Google Cloud Run.

For detailed deployment instructions, including prerequisites, infrastructure
provisioning, and application deployment, please refer to the
[Infrastructure Documentation](infrastructure/README.md).

## API Reference

For detailed information, please refer to the interactive OpenAPI documentation
available at `http://localhost:8005/docs` when running locally, or at
`$SERVICE_URL/docs` if deployed on the cloud. The list below provides a
high-level overview of the API endpoints.

### Metrics Configuration

*   **`POST /api/v2/generate_metrics_config`**

    *   Compiles JSON configuration to `MetricsConfig` proto.
    *   **Params**: `ignore_validation` (bool).
    *   **Accepts**: `application/json`.
    *   **Returns**: `application/x-protobuf` or `text/x-protobuf`.

*   **`POST /api/v2/validate_metrics_config`**

    *   Validates a configuration without generating the full binary.
    *   **Params**: `return_config` (bool).

*   **`POST /api/v2/get_file_descriptor_set`**

    *   Returns the `FileDescriptorSet` necessary to decode the output reports
        of a specific configuration.

### Vehicle Signal Catalogs

*   **`POST /api/v2/vs/`**: Upload/Update a catalog version.
*   **`GET /api/v2/vs/`**: List available catalog versions.
*   **`DELETE /api/v2/vs/{version}`**: Delete a catalog version.

### Service Health

*   **`GET /health`**: Returns 200 OK if healthy.
