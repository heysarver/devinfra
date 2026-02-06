package cmd

import "embed"

// embeddedTemplatesFS contains project templates for new project creation.
// The all: prefix ensures dotfiles like .env.example.tpl are included.
//
//go:embed all:embed/templates
var embeddedTemplatesFS embed.FS
