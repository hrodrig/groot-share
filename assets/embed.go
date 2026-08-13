package assets

import "embed"

// FaviconsFS holds tab icons embedded at build time.
//
//go:embed favicons/*
var FaviconsFS embed.FS
