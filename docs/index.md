## Important note for Graylog OSS

Graylog OSS does not include built‑in "stream backups" or a one‑click "LDAP user sync". This provider documents and enables recommended approaches to implement these needs:

- Graylog Stream Backups (Graylog backups): use OpenSearch snapshot repositories to back up the indices that store your Streams’ data. See: [graylog_opensearch_snapshot_repository](resources/graylog_opensearch_snapshot_repository) with FS/S3 examples.
- Graylog LDAP Integration — sync LDAP groups to Graylog: use [graylog_ldap_group_members](data-sources/graylog_ldap_group_members) to read LDAP group members, then create/update users and assign roles with Terraform. Grant per‑stream access using role‑based permissions.

Search hints (SEO): graylog stream backups, graylog backups, graylog OpenSearch snapshots, graylog ldap integration, graylog sync ldap groups to graylog, graylog ldap groups sync, graylog user sync ldap.

## What's new in 0.3.0

- OpenSearch snapshot repositories to enable backups of data written by your Streams (index snapshots). See resource: [graylog_opensearch_snapshot_repository](resources/graylog_opensearch_snapshot_repository).
- LDAP group members data source to safely drive user sync/role mapping from your directory. See data source: [graylog_ldap_group_members](data-sources/graylog_ldap_group_members).

Import supports IDs and readable keys where applicable. The provider recognizes UUID and 24-hex (Mongo ObjectID) automatically. For titles, you may also use the explicit `title:` prefix.

Supported import shortcuts:
- `graylog_user` — by `username` (fallback: by `id`)
- `graylog_role` — by `name` (fallback: by `id`)
- `graylog_stream` — by `title` or `id` (UUID/24-hex); ambiguous titles cause an error
- `graylog_dashboard` — by `title` or `id` (UUID/24-hex); ambiguous titles cause an error

Examples:

```bash
# By ID (UUID or 24-hex)
terraform import graylog_stream.s 5f3c2a0b9e0f1a2b3c4d5e6f

# By exact title (explicit prefix to be unambiguous)
terraform import graylog_stream.s "title:errors"

# Dashboard by title
terraform import graylog_dashboard.d "title:Ops overview"

# User by username
terraform import graylog_user.u alice

# Role by name
terraform import graylog_role.r read_only
```

If multiple resources share the same title, import by ID to disambiguate.

## Navigation by area

- Streams
  - Resources: [graylog_stream](resources/graylog_stream), [graylog_stream_permission](resources/graylog_stream_permission), [graylog_stream_output_binding](resources/graylog_stream_output_binding)
  - Data sources: [graylog_streams](data-sources/graylog_streams)
- Inputs & Outputs
  - Resources: [graylog_input](resources/graylog_input), [graylog_output](resources/graylog_output)
- Index Sets
  - Resources: [graylog_index_set](resources/graylog_index_set)
- Pipelines
  - Resources: [graylog_pipeline](resources/graylog_pipeline)
- Dashboards
  - Resources: [graylog_dashboard](resources/graylog_dashboard), [graylog_dashboard_widget](resources/graylog_dashboard_widget), [graylog_dashboard_permission](resources/graylog_dashboard_permission)
- Alerts & Events
  - Resources: [graylog_alert](resources/graylog_alert), [graylog_event_notification](resources/graylog_event_notification)
  - Data sources: [graylog_event_notifications](data-sources/graylog_event_notifications)
- Users & Security
  - Resources: [graylog_user](resources/graylog_user), [graylog_role](resources/graylog_role), [graylog_ldap_setting](resources/graylog_ldap_setting)
  - Data sources: [graylog_user](data-sources/graylog_user), [graylog_users](data-sources/graylog_users), [graylog_ldap_group_members](data-sources/graylog_ldap_group_members)
- OpenSearch & Backups
  - Resources: [graylog_opensearch_snapshot_repository](resources/graylog_opensearch_snapshot_repository)

## Environment variables (all supported)

Provider configuration also accepts the following ENV variables (used when an attribute is unset):

- `GRAYLOG_URL`
- `GRAYLOG_AUTH_METHOD`
- `GRAYLOG_USERNAME`
- `GRAYLOG_PASSWORD`
- `GRAYLOG_API_TOKEN`
- `GRAYLOG_API_TOKEN_PASSWORD`
- `GRAYLOG_BEARER_TOKEN`
- `GRAYLOG_TOKEN` (legacy base64 of `user:pass`)
- `GRAYLOG_INSECURE` (set to `1` to skip TLS verification)
- `GRAYLOG_CA_BUNDLE`
- `GRAYLOG_CLIENT_CERT`
- `GRAYLOG_CLIENT_KEY`
- `GRAYLOG_TIMEOUT` (e.g. `30s`, `1m`)
- `GRAYLOG_MAX_RETRIES`
- `GRAYLOG_RETRY_WAIT` (e.g. `1s`, `2s`)
- `GRAYLOG_LOG_LEVEL` (`TRACE`/`DEBUG`/`INFO`/`WARN`/`ERROR`)

Additionally, some features talk to OpenSearch directly (e.g., snapshot repositories). Configure OpenSearch via ENV when provider attributes are unset:

- `OPENSEARCH_URL`
- `OPENSEARCH_INSECURE` (set to `1` to skip TLS verification)

See the resource `graylog_opensearch_snapshot_repository` for details.

## List Data Sources (V1)

For convenient lookups and iteration you can use list data sources that return both `items` and a `title_map`:

```hcl
data "graylog_streams" "all" {}
data "graylog_dashboards" "all" {}
data "graylog_index_sets" "all" {}
data "graylog_event_notifications" "all" {}

output "streams_map" {
  value = data.graylog_streams.all.title_map
}
```

See individual data source pages under `docs/data-sources/` for full attribute references, including:

- `graylog_ldap_group_members` — read‑only helper to list LDAP group members.

## Capability gating (version-aware validations)

Some Graylog features are not available in certain versions/images. The provider performs a light capability probe and will:

- Return a clear error if a managed resource requires an unavailable feature (e.g., classic dashboards CRUD, event notifications on legacy images).
- Avoid silent skips by default.

Examples:

- `graylog_dashboard` (classic dashboards) — create/update will fail with a clear message when the instance does not support legacy dashboards CRUD (common on Graylog 5.x and often 6.x). Use an image/version that exposes classic dashboards or migrate to the supported dashboards experience.
- `graylog_dashboard_widget` and `graylog_dashboard_permission` — gated alongside classic dashboards support.
- `graylog_event_notification` — create/update will fail if Event Notifications APIs are not available in the current image/version.

This behavior helps catch misconfigurations early and keeps plans deterministic.

### Capability table (quick reference)

The following matrix summarizes typical availability across common OSS images. Actual behavior depends on your specific image/build; the provider probes capabilities at runtime and returns clear errors when a feature is missing.

| Feature / Resource(s) | Graylog 5.x | Graylog 6.x | Graylog 7.x |
|---|---|---|---|
| Streams, Index Sets (`graylog_stream`, `graylog_index_set`, permissions for streams) | Yes | Yes | Yes |
| Event Notifications (`graylog_event_notification`, data sources) | Varies (often missing) | Usually available | Available |
| Classic Dashboards CRUD (`graylog_dashboard`, `graylog_dashboard_widget`, `graylog_dashboard_permission`) | Often unavailable | Often unavailable | Varies by image (available on images exposing legacy dashboards) |

Notes:
- “Varies/Often unavailable” reflects differences between images/builds; rely on provider capability checks rather than assuming support.
- When a capability is missing, the provider fails fast on Create/Update with a clear diagnostic and a short hint.
- You may still import/read existing objects where read endpoints are available; gating applies to Create/Update paths.