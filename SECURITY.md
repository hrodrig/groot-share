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

To make the external URL fully deterministic and avoid relying on
proxy-overwritten headers, set `GFS_BASE_URL` (e.g.
`https://share.example.com`). When set, gfs uses it verbatim for every share
link it returns and ignores `X-Forwarded-Proto` / `Host` from the request.
This is the fail-closed path; the env-var value must be an absolute
`http(s)://` URL with a host, or gfs refuses to start.

## Client IP (audit / access log)

gfs records the value of `r.RemoteAddr` only — it does **not** read
`X-Forwarded-For` or any other proxy header. On the standard deployment the
reverse proxy terminates the client connection, so the IP written to the audit
log and the access log is the **proxy's** IP, not the end client's. If you need
the true client IP, configure your proxy to pass it and gate on that being a
trusted hop; gfs itself performs no such parsing today. (There is no way to
make `RemoteAddr` a client IP without either terminating TLS at gfs or teaching
gfs to trust a proxy hop.)

## Audit (fail-open)

`InsertAudit` failures are logged at error level and then ignored — the primary
operation (upload, download, delete, or an admin user/API-key mutation) always
proceeds. This is a deliberate **fail-open** trade-off: an audit-log outage
must not take down the file service it is meant to watch. It means a
persistent `audit` table failure could, in principle, let a write go
unrecorded. Hardening admin actions so their audit record is required before
the action completes is tracked as a follow-up.

## Supported versions

| Version | Supported |
| ------- | --------- |
| Latest release | Yes |
| Older releases | No — upgrade |

## Reporting a vulnerability

**Do not open a public issue** for undisclosed vulnerabilities.

- Preferred: GitHub Security Advisories on this repository.
- Alternatively: contact the maintainer via [github.com/hrodrig](https://github.com/hrodrig).
