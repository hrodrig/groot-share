package core

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/hrodrig/groot-share/internal/store"
)

func (c *Core) UserRecord(ctx context.Context, u store.User) (store.UserRecord, error) {
	items, err := c.Store.ListUsers(ctx)
	if err != nil {
		return store.UserRecord{}, err
	}
	for _, rec := range items {
		if rec.ID == u.ID {
			return rec, nil
		}
	}
	return store.UserRecord{User: u}, nil
}

func UserJSON(u store.UserRecord) map[string]any {
	out := map[string]any{
		"id":       u.ID,
		"username": u.Username,
		"name":     u.Name,
		"role":     u.Role,
		"active":   u.Active,
	}
	if !u.CreatedAt.IsZero() {
		out["created_at"] = u.CreatedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func ParseUserID(raw string) (int64, error) {
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return 0, errors.New("empty id")
	}
	return strconv.ParseInt(raw, 10, 64)
}
