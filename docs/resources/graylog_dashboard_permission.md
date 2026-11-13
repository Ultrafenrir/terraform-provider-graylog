---
page_title: "graylog_dashboard_permission Resource"
subcategory: "Dashboards"
description: |-
  Manages role-based permissions (read/edit/share) for a specific Graylog Dashboard.
---

# graylog_dashboard_permission (Resource)

Manages role-based access to a dashboard. The resource adds or removes permissions for only the selected dashboard and leaves the role's other permissions unchanged.

> Capability note: create and update require classic dashboard CRUD and permission APIs. On images without legacy dashboard support, the provider fails early with a clear error. See the compatibility table in the provider overview.

## Example Usage

```hcl
resource "graylog_dashboard_permission" "d_readwrite" {
  role_name    = "Reader"
  dashboard_id = graylog_dashboard.ops.id
  actions      = ["read", "edit"]
}
```

## Argument Reference

- `role_name` (String, Required) — Name of the role receiving the permissions.
- `dashboard_id` (String, Required) — Dashboard ID.
- `actions` (List of String, Optional) — Any of `read`, `edit`, and `share`. Defaults to `["read"]`.

## Attribute Reference

- `id` (String) — Synthetic ID in `role_name/dashboard_id` format.

## Import

Import with the composite identifier `role_name/dashboard_id` or `role_name:dashboard_id`:

```bash
terraform import graylog_dashboard_permission.d_readwrite Reader/5f1e7d1c2a3b4c5d6e7f8a9b
# or
terraform import graylog_dashboard_permission.d_readwrite Reader:5f1e7d1c2a3b4c5d6e7f8a9b
```

## Notes

- The resource manages only permissions for the selected `dashboard_id`; all other role permissions remain unchanged.
- Permissions written by the provider use `dashboards:<action>:<dashboard_id>` format.
