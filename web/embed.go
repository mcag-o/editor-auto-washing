package web

import "embed"

// Static contains the hosted admin frontend assets.
//
//go:embed static/* dist/* dist/ui/assets/*
var Static embed.FS
