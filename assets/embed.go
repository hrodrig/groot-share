package assets

import "embed"

// FaviconsFS holds tab icons embedded at build time.
//
//go:embed favicons/*
var FaviconsFS embed.FS

// UIFS holds login-gate artwork (cropped crate + wordmark JPEG).
//
//go:embed ui/*
var UIFS embed.FS
