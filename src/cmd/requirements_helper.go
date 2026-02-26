package cmd

import (
	"strings"
	"xe/src/internal/project"
)

func requirementToDepName(requirement string) string {
	name := strings.TrimSpace(requirement)
	if name == "" {
		return ""
	}
	if idx := strings.Index(name, "["); idx >= 0 {
		name = name[:idx]
	}
	if idx := strings.IndexAny(name, " <>=!~;"); idx >= 0 {
		name = name[:idx]
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	return project.NormalizeDepName(name)
}
