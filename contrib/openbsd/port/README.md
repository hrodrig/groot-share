# OpenBSD port — local testing

1. From repo root: `make port-openbsd-sync && make man-sync && make dist-openbsd`
2. Copy `dist/gfs_v*_openbsd_*.tar.gz` into DISTDIR (or `MASTER_SITES=file:///.../`)
3. In the port directory: `make makesum && make install`
