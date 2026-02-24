package cmd

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/heysarver/devinfra/internal/config"
)

// pickProject shows an interactive project selector. Returns the chosen project name.
// Returns ("", nil) if the user cancels (Esc/Ctrl+C) — callers check for empty string.
// Returns an error only if the registry cannot be loaded or is empty.
func pickProject(prompt string) (string, error) {
	reg, err := config.LoadRegistry()
	if err != nil {
		return "", err
	}
	names := reg.List()
	if len(names) == 0 {
		return "", fmt.Errorf("no projects registered; run 'di new' or 'di add' first")
	}

	var name string
	sel := huh.NewSelect[string]().
		Title(prompt).
		Options(huh.NewOptions(names...)...).
		Value(&name)

	if err := huh.NewForm(huh.NewGroup(sel)).Run(); err != nil {
		if errors.Is(err, huh.ErrUserAborted) {
			return "", nil // caller checks for empty string → "Cancelled."
		}
		return "", err
	}
	return name, nil
}
