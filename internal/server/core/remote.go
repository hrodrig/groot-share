package core

import "github.com/hrodrig/groot-share/internal/server/mw"

func RemoteIP(remoteAddr string) string { return mw.RemoteIP(remoteAddr) }
