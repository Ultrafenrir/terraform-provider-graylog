# Graylog Provider Compatibility Report

Verified on November 18, 2026.

## Version tracks

| Track | Graylog versions | Purpose |
|---|---|---|
| Production | 7.1.12, 7.2.2 | Release-supported versions for current installations |
| EOL regression | 5.2.12, 6.0.14, 6.1.16, 6.2.14, 6.3.15, 7.0.16 | Backward compatibility for existing provider users |

Older versions in the regression track are not recommended for new
installations. The split follows Graylog's published release-support dates.

The live matrix selects a compatible MongoDB and OpenSearch version for each
Graylog release. CI runs every full matrix entry on the main branch and a
representative production/regression subset on pull requests.

## Resource coverage

The acceptance suite exercises every registered resource whose API is exposed
by the corresponding OSS image. Mutable lifecycle coverage includes explicit
updates and independent live API assertions for alerts, dashboards, dashboard
permissions, event notifications, outputs, lookup caches, lookup adapters,
lookup tables, cluster configuration, pipelines, index sets, inputs, streams,
roles, users, permissions, bindings, and snapshot repositories.

Known legacy boundaries:

- `graylog_dashboard_widget` uses the unavailable classic-widget endpoint and
  is deprecated in both schema and documentation.
- `graylog_ldap_setting` uses the pre-4.0 LDAP settings endpoint and is
  deprecated in favor of `graylog_auth_backend` and
  `graylog_auth_backend_activation`.
- `graylog_event_notification` is supported on Graylog 6 and newer. Graylog 5
  receives an early diagnostic because its notification payload is
  incompatible.

## Upgrade coverage

`make test-migration` verifies that one Terraform state survives the incremental
Graylog 5.2 -> 6.0 -> 6.1 -> 6.2 -> 6.3 -> 7.0 -> 7.1 -> 7.2 path without
resource destruction. It does not claim to upgrade production databases.

`make test-data-upgrade` is a separate opt-in procedure. It stores marker data
in named MongoDB and OpenSearch volumes, upgrades both dependencies and Graylog
incrementally, and verifies the data after the final step. A dedicated manual
CI workflow runs this slower and higher-risk scenario independently of normal
pull-request validation.
