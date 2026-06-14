package frontend

import "embed"

// DistFS contains the production SPA build output (web npm run build).
//go:embed dist/*
var DistFS embed.FS
