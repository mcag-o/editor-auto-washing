package web

import "embed"

// Static contains the hosted admin frontend assets.
//
//go:embed static/*
var Static embed.FS
