# Testing Guide

## Test tracks

The live suite intentionally distinguishes currently release-supported
versions from compatibility coverage for existing installations:

- Production matrix: Graylog 7.1 and 7.2.
- EOL regression matrix: Graylog 5.2, 6.0-6.3, and 7.0.

The split follows Graylog's published release-support dates as of November
2026. EOL regression coverage is not a recommendation for new installations.

```bash
make test-integration-production
make test-acc-production
make test-integration-regression
make test-acc-regression
```

`make test-integration-all` and `make test-acc-all` run both tracks.

## Fast checks

```bash
make lint
make test-unit
make build-quick
```

Run one live version or one test with:

```bash
make test-integration-one GRAYLOG_VERSION=7.2.2 RUN=TestIntegration_Stream
make test-acc-one GRAYLOG_VERSION=7.2.2 RUN=TestAccDashboard_basic
```

Acceptance tests require Terraform CLI 1.6 or newer. Live tests require Docker,
Docker Compose, and `jq`.

## Acceptance-test requirements

- Live suites must not use `httptest` or a replaced HTTP transport.
- Mutable resources must include an update step and read the resulting values
  through a separate Graylog client.
- Effect resources must check their external result, such as role permissions
  or stream/output bindings.
- A repeated configuration must produce an empty plan where applicable.
- The test command uses `-count=1` so results are never reused across Graylog
  matrix entries.

The legacy `graylog_dashboard_widget` and `graylog_ldap_setting` resources are
not part of the OSS live matrix because their endpoints are not exposed by the
tested images. Both resources remain registered for state compatibility and
carry schema-level deprecation messages.

`graylog_event_notification` requires Graylog 6 or newer. Graylog 5 uses an
incompatible payload and receives an explicit provider diagnostic.

## Terraform state migration

```bash
make test-migration
```

This test preserves one Terraform state while Graylog moves through every
minor release from 5.2 to 7.2. It rejects any plan containing a delete action.
It uses `username` and `password` provider authentication and should emit no
deprecated-provider or read-only-attribute warnings.

This is a provider/state compatibility test. It deliberately keeps one
transitional MongoDB/OpenSearch pair and is not a database upgrade test.

## Persistent data upgrade

```bash
make test-data-upgrade
```

This opt-in test creates marker data in persistent MongoDB and OpenSearch
volumes, upgrades MongoDB, OpenSearch, and every Graylog minor step, then reads
both markers from the final stack. It is destructive to the isolated Compose
project and is intentionally excluded from normal pull-request CI. Run it via
the manual `Persistent data upgrade` GitHub Actions workflow when dependency or
upgrade logic changes.

Set `KEEP_UPGRADE_STACK=1` to retain the final stack for diagnostics.

## Pre-release validation

```bash
make test-pre-release
```

This runs unit tests, both live compatibility tracks, and the Terraform state
migration. Run the persistent data upgrade workflow separately when a release
changes supported Graylog, MongoDB, or OpenSearch versions.
