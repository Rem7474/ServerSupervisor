# Security Policy

## Supported versions

Only the latest published release (`ghcr.io/rem7474/serversupervisor:latest` / the most
recent `vX.Y.Z` tag) receives security fixes. Please update before reporting an issue that
might already be fixed.

## Reporting a vulnerability

Please **do not** open a public GitHub issue for a security vulnerability.

Instead, use [GitHub Security Advisories](https://github.com/Rem7474/ServerSupervisor/security/advisories/new)
to report privately. Include:

- A description of the vulnerability and its impact
- Steps to reproduce (PoC if possible)
- Affected version/commit

You should get an initial response within a few days. Once a fix is confirmed, it will be
released as soon as reasonably possible and credited in the release notes (unless you'd
rather stay anonymous).

## Scope

This policy covers the ServerSupervisor server, agent, and their official Docker images
(`ghcr.io/rem7474/serversupervisor`). Third-party dependencies are monitored separately via
Dependabot and the CI security workflow (`.github/workflows/security.yml`).

Given ServerSupervisor's threat model — a self-hosted admin tool that executes a
server-enforced whitelist of remote commands (see `CLAUDE.md`) — reports involving
authentication/authorization bypass, RBAC/host-permission gaps, command whitelist escapes,
or secret handling (JWT, API keys, Proxmox tokens) are especially high priority.
