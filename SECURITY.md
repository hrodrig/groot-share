# Security policy — gfs

## Scope

Web/API door for groot `.tar.gz` archives on a VPS (optional S3-compatible backend).
Treat user passwords, api_keys, and bucket credentials as sensitive. Never log them
(see `docs/SPECIFICATIONS.md`).

On topology `vps-s3`, download and delete are scoped to `GFS_S3_PREFIX` so gfs cannot
read or remove arbitrary objects elsewhere in a shared bucket.

`POST /login` is rate-limited in-process (default `GFS_LOGIN_RATE_LIMIT=20/1m` per
client IP and per username).

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
