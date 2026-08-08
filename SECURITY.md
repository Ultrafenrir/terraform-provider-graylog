# Security Policy

## Supported Versions

Security fixes are provided for the latest released version of the provider.
Older releases may receive a fix at the maintainers' discretion. Before
reporting a vulnerability, verify that it is reproducible with the latest
release.

| Version | Supported |
| --- | --- |
| Latest release | Yes |
| Older releases | No |

## Reporting a Vulnerability

Do not disclose suspected vulnerabilities in a public issue, discussion, pull
request, or other public channel.

Submit a private report through
[GitHub private vulnerability reporting](https://github.com/Ultrafenrir/terraform-provider-graylog/security/advisories/new).
If private reporting is unavailable, contact the maintainer through the
[repository owner's GitHub profile](https://github.com/Ultrafenrir) and request
a private communication channel without including vulnerability details.

Please include, when possible:

- A clear description of the vulnerability and its impact
- Affected provider and Graylog versions
- Reproduction steps or a minimal proof of concept
- Required configuration or environment conditions
- Any known mitigations or suggested fixes
- Whether the vulnerability has been disclosed elsewhere

Never include real credentials, Terraform state, customer data, or other
secrets in a report. Use synthetic and sanitized examples.

## What to Expect

The maintainers aim to acknowledge a report within five business days. After
triage, the reporter will be informed whether the issue is accepted, requires
more information, or is out of scope. Progress updates will normally be provided
at least every ten business days while an accepted report remains open.

Fix and disclosure timelines depend on severity, complexity, and coordination
with Graylog or other upstream projects. Please allow a reasonable remediation
period before public disclosure. The maintainers will coordinate disclosure and
credit with the reporter unless anonymity is requested.

## Scope

Examples of in-scope reports include provider behavior that can expose secrets,
bypass intended authorization, corrupt Terraform state in a security-relevant
way, or send sensitive data to an unintended destination.

General support requests, insecure user configuration, and vulnerabilities that
exist only in an upstream dependency or Graylog itself are normally out of
scope. Upstream findings are still welcome when provider behavior materially
increases their impact.
