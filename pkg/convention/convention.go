package convention

import "path/filepath"

// ScaffoldDir is the root directory for scaffold-tool configs/templates.
// Org layers may override this at startup.
var ScaffoldDir = ".scafctl"

// ScaffoldConfigsPrefix returns the path prefix for scaffold config files.
func ScaffoldConfigsPrefix() string {
	return filepath.Join(ScaffoldDir, "configs") + "/"
}

// ScaffoldTemplatesPrefix returns the path prefix for scaffold template files.
func ScaffoldTemplatesPrefix() string {
	return filepath.Join(ScaffoldDir, "templates") + "/"
}
