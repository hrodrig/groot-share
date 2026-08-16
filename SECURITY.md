# Security policy — gfs

## Scope

Web/API door for groot `.tar.gz` archives on a VPS (optional S3-compatible backend).
Treat user passwords, api_keys, and bucket credentials as sensitive. Never log them
(see `docs/SPECIFICATIONS.md`).

On topology `vps-s3`, download and delete are scoped to `GFS_S3_PREFIX` so gfs cannot
read or remove arbitrary objects elsewhere in a shared bucket.

`POST /login` is rate-limited in-process (default `GFS_LOGIN_RATE_LIMIT=20/1m` per
client IP and per username).

Password changes invalidate all sessions for that user (Settings, `PATCH /v1/me`, or
admin password patch).

## Deployment (TLS / proxy)

gfs does not terminate TLS itself. Put a **trusted** reverse proxy in front (see
[groot-share-selfhosted](https://github.com/hrodrig/groot-share-selfhosted)).

Absolute links (copy-download URL) use `Request.Host` and, when not serving TLS
directly, `X-Forwarded-Proto`. The proxy must overwrite those headers; do **not**
expose gfs to untrusted clients that can set `Host` / `X-Forwarded-*`.

## Supported versions

| Version | Supported |
| ------- | --------- |
| Latest release | Yes |
| Older releases | No — upgrade |

## Reporting a vulnerability

**Do not open a public issue** for undisclosed vulnerabilities.

- Preferred: GitHub Security Advisories on this repository.
- Alternatively: contact the maintainer via [github.com/hrodrig](https://github.com/hrodrig).
