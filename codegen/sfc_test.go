package codegen

import (
	"path/filepath"
	"strings"
	"testing"

	uicompiler "github.com/karty-game/karty-ui/compiler"
)

func TestSFCFileNameBecomesGeneratedComponentFunction(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "samples", "sfc-demo")

	components, err := uicompiler.LoadProjectWithTheme(root,
		[]uicompiler.Source{{Name: "ui.menu", Source: "ui/views/menu.kui"}},
		[]uicompiler.LayoutSource{{Source: "ui/layouts/main-menu.kui"}}, "")
	if err != nil {
		t.Fatal(err)
	}

	files, err := UIClientFiles(components, "example.com/sfc-demo")
	if err != nil {
		t.Fatal(err)
	}

	component, ok := files[components[0].Source]
	if !ok || !strings.Contains(string(component), "func Menu(args MenuProps)") {
		t.Fatalf("generated file-name component function missing: %s", component)
	}
}
