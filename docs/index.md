---
page_title: "Graylog Terraform Provider — Graylog automation with Terraform"
subcategory: ""
description: |-
  Terraform Graylog provider to automate Graylog operations (streams with rules, inputs with extractors, index sets, pipelines, dashboards, alerts/Event Definitions, notifications). Works with Graylog v5, v6, v7. Keywords: terraform graylog provider, graylog terraform, terraform graylog, graylog automation, Graylog operation automation.
---

# Graylog Terraform Provider

The Terraform provider for Graylog manages:
- Streams (with stream rules)
- Inputs (with flexible configuration and optional extractors)
- Index sets
- Pipelines (classic)
- Dashboards (classic)
- Alerts (Event Definitions)
- Event Notifications (email/http/slack/pagerduty)
- Users (local)
- Dashboard Widgets (classic dashboards)

## Example Usage (Terraform Graylog)

```hcl
terraform {
  required_providers {
    graylog = {
      source  = "Ultrafenrir/graylog"
      version = "~> 0.1"
    }
  }
}

provider "graylog" {
  url   = "http://localhost:9000/api"
  token = "${base64encode("admin:admin")}" # or an API token
}
```

See resource and data source pages for detailed examples of Graylog automation with Terraform.

## Authentication
- `url` — Graylog API base URL (often `http://<host>:9000/api`).
- `token` — Basic auth encoded as base64(`user:pass`) or an API token.

## Compatibility
The provider targets Graylog v5, v6, and v7. For Graylog v6/v7, API requests automatically use the `/api` prefix where required. Designed for repeatable Graylog operation automation with Terraform.

## Supported Graylog versions

- Graylog 5.x
- Graylog 6.x
- Graylog 7.x

Validation matrix: integration, acceptance, and migration tests are executed in CI across all listed major versions to ensure ongoing compatibility.
