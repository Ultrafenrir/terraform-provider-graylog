# Contributing to terraform-provider-graylog

Thank you for helping improve the Graylog Terraform provider. Contributions of
bug fixes, tests, documentation, and new provider features are welcome.

By participating in this project, you agree to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).

## Before You Start

- Search existing issues and pull requests to avoid duplicate work.
- For a bug, include a minimal reproducible Terraform configuration and the
  relevant Terraform, provider, and Graylog versions.
- For a substantial feature or breaking change, open an issue first so the
  design and compatibility impact can be discussed.
- Report vulnerabilities according to [SECURITY.md](SECURITY.md), never in a
  public issue.

## Development Requirements

- Go 1.24 or later
- Terraform CLI
- GNU or BSD `make`
- Docker with Docker Compose for integration and acceptance tests
- `curl` for local Graylog readiness checks

Fork the repository, create a focused branch, and install dependencies:

```shell
git clone https://github.com/YOUR-USER/terraform-provider-graylog.git
cd terraform-provider-graylog
go mod download
```

## Making Changes

- Keep changes focused and preserve backward compatibility whenever possible.
- Follow existing provider schema and client conventions.
- Add or update unit tests for behavior changes and bug fixes.
- Add integration coverage when behavior depends on the Graylog API.
- Update documentation and examples when user-facing behavior changes.
- Never commit credentials, tokens, Terraform state, or unsanitized logs.

Format and validate the code before submitting a pull request:

```shell
make fmt
make lint
make test-unit
```

To run integration tests against the complete Graylog stack for all supported
major versions:

```shell
make test-integration-all
```

For faster iteration, run a selected integration test against one Graylog
version:

```shell
make GRAYLOG_VERSION=7.0 \
  PKG=./internal/provider \
  RUN='TestIntegrationInput' \
  test-integration-one
```

Run the full pre-release suite for changes with broad compatibility impact:

```shell
make test-pre-release
```

Integration tests start containers and can take several minutes. Ensure the
stack is stopped after an interrupted run with `make graylog-down`.

## Pull Requests

A pull request should:

- Explain the problem and why the proposed solution is appropriate.
- Describe any user-visible or state migration impact.
- Link related issues.
- Include tests that would fail without the change.
- Pass formatting, unit tests, and the relevant integration matrix.
- Update `README.md`, `docs/`, examples, or `CHANGELOG.md` when applicable.

Maintainers may request changes to keep provider behavior consistent across
supported Graylog versions.

## Reporting Bugs

Use the GitHub bug report form and include:

- Terraform, provider, Graylog, and operating system versions
- A minimal sanitized Terraform configuration
- Exact reproduction steps
- Expected and actual behavior
- Relevant sanitized logs or API responses

Remove passwords, API tokens, private URLs, Terraform state, and other sensitive
data before submitting an issue.
