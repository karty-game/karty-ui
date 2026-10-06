package uicompiler

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	ui "github.com/karty-game/karty-ui/schema"
)

// Validate the editor's handwritten catalog against the compiler and schema so
// completions cannot silently drift to unsupported properties, units or widgets.
func TestEditorStyleCatalogMatchesCompiler(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../editors/vscode/src/catalog.json")
	if err != nil {
		t.Fatal(err)
	}

	var catalog struct {
		Properties map[string]struct {
			Targets []string `json:"targets"`
			Detail  string   `json:"detail"`
			Values  []string `json:"values"`
			Aliases []string `json:"aliases"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		t.Fatal(err)
	}

	names := supportedStylePropertyNames()
	if len(catalog.Properties) != len(names) {
		t.Fatalf("editor property count %d != compiler %d", len(catalog.Properties), len(names))
	}

	for _, alias := range []string{"direction", "align", "justify", "grow"} {
		if !slices.Contains(catalog.Properties[canonicalStyleProperty(alias)].Aliases, alias) {
			t.Fatal("missing compact editor alias", alias)
		}
	}

	theme := DefaultTheme()
	theme.Images["example"] = ui.Image{Scope: ui.ImageScopeGame, Name: "example"}
	kinds := []string{"root", "panel", "label", "image", "button", "list", "checkbox", "input", "slider", "combo", "tabs", "tab"}

	for name, entry := range catalog.Properties {
		if !slices.Contains(names, name) || entry.Detail == "" {
			t.Fatal("unknown or undocumented property", name)
		}

		for _, alias := range entry.Aliases {
			if canonicalStyleProperty(alias) != name {
				t.Fatal("incorrect alias", alias, name)
			}
		}

		values := entry.Values
		if len(values) == 0 {
			values = []string{"theme.images.example"}
		}

		for _, value := range values {
			var style ui.Style
			if err := applyStyleProperty(&style, name, value, theme); err != nil {
				t.Fatalf("editor %s: %s: %v", name, value, err)
			}

			for _, kind := range kinds {
				advertised := slices.Contains(entry.Targets, kind)
				if accepted := stylePropertyValid(style, kind); accepted != advertised {
					t.Errorf("editor %s: %s on %s: advertised %v, accepted %v", name, value, kind, advertised, accepted)
				}
			}
		}
	}
}
