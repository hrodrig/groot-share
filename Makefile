# groot-trigger — BSD Make stub: forwards to GNU Make.
# On FreeBSD: pkg install gmake

_CHECK := command -v gmake >/dev/null 2>&1 || { echo "This project requires GNU make. On FreeBSD: pkg install gmake"; exit 1; }

all:
	@${_CHECK}
	@gmake help

.DEFAULT:
	@${_CHECK}
	@gmake $@
