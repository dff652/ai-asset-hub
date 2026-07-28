# Security policy

## Supported versions

AI Asset Hub is currently a technical preview. Security fixes are provided for the latest
release only; older releases are not guaranteed to receive backports.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability or accidentally exposed secret.
Use GitHub's private vulnerability reporting:

<https://github.com/dff652/ai-asset-hub/security/advisories/new>

Include the affected version, operating system, reproduction steps, expected impact, and any
suggested mitigation. Remove real credentials, customer data, and private configuration from
the report.

If private vulnerability reporting is unavailable, open a public issue containing no sensitive
details and ask the maintainer for a private contact channel.

## Scope

Reports are especially useful for:

- path traversal, symlink escape, archive extraction, or unintended file overwrite;
- secret disclosure through reports, logs, manifests, packages, or crash output;
- backup, rollback, journal, or file-mode integrity failures;
- untrusted hook, MCP, or asset content being executed unexpectedly;
- release artifact or checksum integrity problems.

The project's security model and trust boundaries are documented in
[docs/security.md](docs/security.md).
