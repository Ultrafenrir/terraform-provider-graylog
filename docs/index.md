---
page_title: "Graylog Provider"
subcategory: ""
description: |-
  Terraform provider for managing Graylog resources: streams (with rules), inputs (with extractors), index sets, pipelines, dashboards, and alerts (Event Definitions). Supports Graylog v5, v6, and v7.
---

# Graylog Provider

The Graylog provider manages:
- Streams (with stream rules)
- Inputs (with flexible configuration and optional extractors)
- Index sets
- Pipelines (classic)
- Dashboards (classic)
- Alerts (Event Definitions)

## Example Usage

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

See resource and data source pages for detailed examples.

## Authentication
- `url` — Graylog API base URL (often `http://<host>:9000/api`).
- `token` — Basic auth encoded as base64(`user:pass`) or an API token.

## Compatibility
The provider targets Graylog v5, v6, and v7. For Graylog v6/v7, API requests automatically use the `/api` prefix where required.
