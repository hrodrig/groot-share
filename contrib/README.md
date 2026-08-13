# Packaging extras

| Path | Purpose |
|------|---------|
| `contrib/freebsd/` | FreeBSD port skeleton (`sysutils/gfs`) |
| `contrib/openbsd/port/` | OpenBSD port skeleton |
| `contrib/man/man1/` | Manual pages bundled in release tarballs |

From repo root:

```bash
make port-freebsd-sync
make port-openbsd-sync
make man-sync
make dist-freebsd          # FREEBSD_ARCH=amd64|arm64
make dist-openbsd          # OPENBSD_ARCH=amd64|arm64
```
