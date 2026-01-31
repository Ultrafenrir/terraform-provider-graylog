# Graylog Terraform Provider

This provider manages Graylog resources: streams (with rules), inputs (with extractors), index sets, pipelines, dashboards, and alerts (Event Definitions). It targets Graylog v5, v6, and v7. API prefix `/api` is applied automatically for v6/v7 where needed.

## Installation (Terraform Registry)

Add the provider requirement (namespace is based on the GitHub org):

```hcl
terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.1"
    }
  }
}
```

Then configure the provider:

```hcl
provider "graylog" {
  # URL to Graylog API (often http://<host>:9000/api)
  url   = "http://localhost:9000/api"
  # Basic auth (base64 of user:pass) or API token
  token = "<base64(user:pass)>"
}
```

See the `examples/` directory for standalone, copy‑pasteable snippets for each resource type.

## Supported Graylog versions

The provider is tested and supported against the following Graylog major versions:

- Graylog 5.x
- Graylog 6.x
- Graylog 7.x

CI runs both integration and acceptance tests against all listed versions via docker-compose to ensure compatibility.

## Examples

Examples are organized by resource type:
- examples/basic.tf — single file covering provider config and all main resources.
- examples/inputs/*.tf — inputs like Kafka, Syslog UDP, GELF TCP, Beats, Raw TCP/UDP, HTTP JSON. Each uses a flexible `configuration` map and may include `extractors`.
- examples/streams/*.tf — streams with multiple rules (integer `type` enum); includes `inverted` examples.
- examples/pipelines/*.tf — pipelines; `source` contains a full Graylog pipeline definition.
- examples/dashboards/*.tf — classic dashboards (title/description).
- examples/alerts/*.tf — alert/Event Definition examples with pass‑through `config`.

Notes:
- Stream rule `type` is an integer Graylog enum. Values vary by Graylog version (e.g., equals=1, regex=3). Consult your Graylog docs.
- Extractors are passed through as free‑form objects. Prefer a consistent format: either top‑level fields or a single `data` object with extractor payload.

## Docker Compose/dev environment

Use the provided `docker-compose.yml` to run a local Graylog for testing:

```bash
make graylog-up
make graylog-wait
# visit http://127.0.0.1:9000
```

Authentication defaults in examples/tests assume `admin:admin`. You can override environment variables for integration tests:

```bash
URL=http://127.0.0.1:9000/api TOKEN=$(printf 'user:pass' | base64) make test-integration
```

Useful commands:
- `make graylog-up` — start the stack
- `make graylog-wait` — wait for API readiness
- `make graylog-logs` — follow Graylog service logs
- `make graylog-down` — stop and remove the stack

## Importing existing resources

You can import existing Graylog resources by ID:

```bash
terraform import graylog_stream.s <stream_id>
terraform import graylog_input.i <input_id>
terraform import graylog_dashboard.d <dashboard_id>
terraform import graylog_pipeline.p <pipeline_id>
terraform import graylog_alert.a <event_definition_id>
terraform import graylog_index_set.is <index_set_id>
```

After import, run `terraform plan` to see any drift and adjust your configuration.

## Integration tests

Integration tests run against a real Graylog via docker‑compose:
1. Requirements: Docker/Compose; ports 9000 (Graylog UI/API) and 9200 (OpenSearch) available.
2. Run once:
   ```bash
   make test-integration
   ```
   The target brings up Graylog, waits for readiness, then runs `go test` with the `integration` tag.

Note: Integration tests are marked with `//go:build integration` and are not executed by a regular `make test`.

## Releases and publishing

- GitHub Actions build and publish artifacts on tags matching `v*`.
- Artifacts include platform zips, `SHA256SUMS` and `SHA256SUMS.sig` signed with your GPG key.
- To publish a new version: push a tag, for example `git tag v0.1.0 && git push origin v0.1.0`.
