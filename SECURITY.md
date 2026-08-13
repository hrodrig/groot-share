# Security policy — gfs

## Scope

Web/API door for groot `.tar.gz` archives on a VPS (optional S3-compatible backend).
Treat user passwords, api_keys, and bucket credentials as sensitive. Never log them
(see `docs/SPECIFICATIONS.md`).

## Supported versions

| Version | Supported |
| ------- | --------- |
| Latest release (when published) | Yes |
| Older releases | No — upgrade |

Until the first tagged release, treat `develop` as the security surface.

## Reporting a vulnerability

**Do not open a public issue** for undisclosed vulnerabilities.

- Preferred: GitHub Security Advisories on this repository (once the remote exists).
- Alternatively: contact the maintainer via [github.com/hrodrig](https://github.com/hrodrig).
