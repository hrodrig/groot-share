package core

import (
	"net/http"
	"strings"
)

func DownloadID(r *http.Request) string {
	id := strings.Trim(r.PathValue("id"), "/")
	id = strings.TrimSuffix(id, "/file")
	return strings.TrimSuffix(id, "/")
}

func DeleteID(r *http.Request) string {
	return strings.Trim(r.PathValue("id"), "/")
}
