# Changelog

## v0.2.1
- Examples: added a comprehensive set of standalone examples under the `examples/` directory:
  - Inputs: Beats, GELF UDP, Raw TCP, Raw UDP, HTTP JSON; all using flexible `configuration` maps and optional `extractors`.
  - Streams: additional example with multiple rule types and inverted rules.
  - Pipelines: multi-stage pipeline example with notes on stream connection.
  - Alerts: alternative threshold-style `config` example.
- Docs: README updated with an Examples section, docker-compose usage, provider configuration, import tips, and notes about stream rule type enum and extractors format.

## v0.2.0
- Expanded resource support:
  - graylog_pipeline: create/read/update/delete (classic pipelines) with the `source` field.
  - graylog_dashboard: basic classic dashboards (title/description) CRUD.
  - graylog_alert: Event Definitions with flexible `config` (`map(dynamic)`) and `notification_ids`.
- Stream rules support via dedicated APIs:
  - Client methods: `ListStreamRules`, `CreateStreamRule`, `DeleteStreamRule`.
  - `graylog_stream` resource: `rule` block (id, field, type:int, value, inverted, description); full resync on Update.
- Inputs:
  - Flexible `configuration` as `map(dynamic)` for all input types (including Kafka).
  - Support for `extractors` as a list of free-form objects (pass-through) with CRUD via API.

## v0.1.0
- Initial skeleton with streams, inputs, index sets.