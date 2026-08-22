# OpenBSD port for gfs
# Copy to /usr/ports/sysutils/gfs/ and run: make makesum
# Submit: send diff to ports@openbsd.org

COMMENT =	authenticated web and API door for groot diagnostic archives
DISTNAME =	gfs_v0.5.0_openbsd_${MACHINE_ARCH:S/aarch64/arm64/}
PKGNAME =	gfs-0.5.0
CATEGORIES =	sysutils
HOMEPAGE =	https://github.com/hrodrig/groot-share
MAINTAINER =	hrodrig@usb.ve

# MIT
PERMIT_PACKAGE =	Yes
PERMIT_DISTFILES =	Yes

# Local test: copy tarball to DISTDIR or
#   make fetch MASTER_SITES=file:///absolute/path/to/dir/
MASTER_SITES =	https://github.com/hrodrig/groot-share/releases/download/v0.5.0/
DISTFILES =	gfs_v0.5.0_openbsd_${MACHINE_ARCH:S/aarch64/arm64/}.tar.gz
EXTRACT_SUFX =	.tar.gz

ONLY_FOR_ARCHS =	amd64 arm64 aarch64
NO_BUILD =	Yes
NO_TEST =	Yes
PKG_ARCH =	*

do-install:
	${INSTALL_PROGRAM} ${WRKDIR}/gfs ${PREFIX}/bin/gfs
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/gfs.1 ${PREFIX}/man/man1/gfs.1
	${INSTALL_DATA_DIR} ${PREFIX}/share/doc/gfs
	${INSTALL_DATA} ${WRKDIR}/share/doc/gfs/LICENSE ${PREFIX}/share/doc/gfs/
	${INSTALL_DATA} ${WRKDIR}/share/doc/gfs/README.md ${PREFIX}/share/doc/gfs/

.include <bsd.port.mk>
