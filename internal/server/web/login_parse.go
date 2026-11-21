package web

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func parseLogin(r *http.Request) (username, password string, asJSON bool, err error) {
	// A JSON Content-Type marks this as an API login: both the body parse and
	// the response shape (JSON error/body vs HTML redirect) key off the same
	// single check, so they can't disagree. The browser leaves Content-Type as
	// application/x-www-form-urlencoded (ParseForm) despite sending the form.
	asJSON = strings.Contains(r.Header.Get("Content-Type"), "application/json")
	if asJSON {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
		if err = dec.Decode(&body); err != nil {
			return "", "", true, err
		}
		return strings.TrimSpace(body.Username), body.Password, true, nil
	}
	if err = r.ParseForm(); err != nil {
		return "", "", false, err
	}
	return strings.TrimSpace(r.Form.Get("username")), r.Form.Get("password"), false, nil
}
