PORTNAME=	gfs
PORTVERSION=	0.5.1
DISTVERSIONPREFIX=	v
CATEGORIES=	sysutils
# Local test before release: copy the tarball into DISTDIR, or run e.g.
#   make fetch MASTER_SITES=file:///absolute/path/to/dir/
# See README.md "Test with a local distfile".
MASTER_SITES=	https://github.com/hrodrig/groot-share/releases/download/v${PORTVERSION}/
DISTFILES=	gfs_v${PORTVERSION}_freebsd_${ARCH:S/aarch64/arm64/}.tar.gz
EXTRACT_SUFX=	.tar.gz

MAINTAINER=	hrodrig@usb.ve
COMMENT=	Authenticated web and API door for groot diagnostic archives
WWW=		https://github.com/hrodrig/groot-share
LICENSE=	MIT

NO_BUILD=	yes
ONLY_FOR_ARCHS=	amd64 aarch64

do-install:
	${INSTALL_PROGRAM} ${WRKDIR}/gfs ${STAGEDIR}${PREFIX}/bin/gfs
	${MKDIR} ${STAGEDIR}${PREFIX}/share/man/man1
	${INSTALL_MAN} ${WRKDIR}/share/man/man1/gfs.1 ${STAGEDIR}${PREFIX}/share/man/man1/
	${MKDIR} ${STAGEDIR}${PREFIX}/share/doc/gfs
	${INSTALL_DATA} ${WRKDIR}/share/doc/gfs/LICENSE ${STAGEDIR}${PREFIX}/share/doc/gfs/
	${INSTALL_DATA} ${WRKDIR}/share/doc/gfs/README.md ${STAGEDIR}${PREFIX}/share/doc/gfs/

.include <bsd.port.mk>
