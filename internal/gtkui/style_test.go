package gtkui

import (
	"os"
	"regexp"
	"sort"
	"testing"
)

// TestStyleColorsAreDefined guards against the silent failure mode where a
// declaration references a colour that was never defined. GTK drops such
// declarations, which previously left the preset popover transparent.
func TestStyleColorsAreDefined(t *testing.T) {
	css, err := os.ReadFile("style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}

	defined := make(map[string]bool)
	for _, match := range regexp.MustCompile(`@define-color\s+([A-Za-z0-9_-]+)`).FindAllStringSubmatch(string(css), -1) {
		defined[match[1]] = true
	}

	var missing []string
	seen := make(map[string]bool)
	for _, match := range regexp.MustCompile(`@([A-Za-z0-9_-]+)`).FindAllStringSubmatch(string(css), -1) {
		name := match[1]
		if name == "define-color" || seen[name] {
			continue
		}
		seen[name] = true
		if !defined[name] {
			missing = append(missing, name)
		}
	}

	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("style.css references undefined colours: %v", missing)
	}
}
