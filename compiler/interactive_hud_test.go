package uicompiler

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	ui "github.com/karty-game/karty-ui/schema"
)

func TestProjectInteractiveHUDChildSchema(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		rootModal     string
		childModal    string
		childMarkup   string
		nested        bool
		rootVersion   uint32
		childVersion  uint32
		middleVersion uint32
	}{
		{"button", "false", "false", `<button onClick={ move }>Move</button>`, false, ui.SchemaInteractionPolish, ui.SchemaInteractionPolish, 0},
		{"list", "false", "false", `<list rows={ []UIRow{} } onClick={ func(uint32) { move() } }/>`, false, ui.SchemaInteractionPolish, ui.SchemaInteractionPolish, 0},
		{"nested", "false", "false", `<button onClick={ move }>Move</button>`, true, ui.SchemaInteractionPolish, ui.SchemaInteractionPolish, ui.SchemaInteractionPolish},
		{"modal child", "false", "true", `<button onClick={ move }>Move</button>`, false, ui.SchemaInteractionPolish, 1, 0},
		{"modal middle", "false", "true", `<button onClick={ move }>Move</button>`, true, ui.SchemaInteractionPolish, 1, ui.SchemaComposition},
		{"modal root", "true", "true", `<button onClick={ move }>Move</button>`, false, ui.SchemaComposition, 1, 0},
		{"noninteractive", "false", "false", `<label>Move</label>`, true, ui.SchemaComposition, 1, ui.SchemaComposition},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			child := "Controls"
			entries := []Source{{Name: "controls", Source: "controls.ui"}}

			sources := map[string]string{
				"controls.ui": `kartui Controls(move func()) { <panel modal="` + test.childModal + `">` + test.childMarkup + `</panel> }`,
			}
			if test.nested {
				child = "Middle"

				entries = append(entries, Source{Name: "middle", Source: "middle.ui"})
				sources["middle.ui"] = `kartui Middle(move func()) { <panel modal="` + test.childModal + `"><Controls props={ move }/></panel> }`
			}

			entries = append(entries, Source{Name: "hud", Source: "hud.ui"})

			sources["hud.ui"] = `kartui HUD(move func()) { <panel modal="` + test.rootModal + `"><` + child + ` props={ move }/></panel> }`
			for name, source := range sources {
				if err := os.WriteFile(filepath.Join(root, name), []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			// Resolve parents both before and after their children in the input.
			versions := map[string]uint32{"HUD": test.rootVersion, "Controls": test.childVersion, "Middle": test.middleVersion}
			assertHUDProjectSchemas(t, root, entries, versions)
			slices.Reverse(entries)
			assertHUDProjectSchemas(t, root, entries, versions)
		})
	}
}

func assertHUDProjectSchemas(t *testing.T, root string, entries []Source, versions map[string]uint32) {
	t.Helper()

	components, err := Load(root, entries)
	if err != nil {
		t.Fatal(err)
	}

	for _, component := range components {
		if component.Template.Version != versions[component.Name] {
			t.Errorf("%s schema=%d, want %d", component.Name, component.Template.Version, versions[component.Name])
		}

		if err := component.Template.ValidateComposition(); err != nil {
			t.Errorf("%s template: %v", component.Name, err)
		}
	}
}

func TestInteractiveHUDDynamicChildSchema(t *testing.T) {
	t.Parallel()

	for _, test := range []struct{ name, body string }{
		{"conditional", "if true {\n<Controls props={ move }/>\n}"},
		{"keyed", "for _, key := range []uint32{1} {\n<Controls props={ move } key={ key }/>\n}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			root, err := Compile("hud.ui", []byte("kartui HUD(move func()) { <panel modal=\"false\">\n"+test.body+"\n</panel> }"))
			if err != nil {
				t.Fatal(err)
			}

			child, err := Compile("controls.ui", []byte(`kartui Controls(move func()) {
<panel modal="false"><button onClick={ move }>Move</button></panel>
}`))
			if err != nil {
				t.Fatal(err)
			}

			components := []Component{root, child}
			if err := resolveChildren(components); err != nil {
				t.Fatal(err)
			}

			if components[0].Template.Version != ui.SchemaInteractionPolish || components[0].Template.Modal {
				t.Fatalf("dynamic child lost the nonmodal interaction policy: %+v", components[0].Template)
			}
		})
	}
}

func TestCompileInteractiveNonmodalHUD(t *testing.T) {
	t.Parallel()

	component, err := Compile("hud.ui", []byte(`kartui HUD(Move func()) {
<panel modal="false"><label>Walk</label><button onClick={ Move }>Forward</button></panel>
}`))
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Modal || component.Template.Back || component.Template.Version != ui.SchemaInteractionPolish ||
		component.Template.Elements[1].Action == 0 {
		t.Fatalf("incorrect HUD input policy: %+v", component.Template)
	}

	if _, err := Compile("bad.ui", []byte(`kartui Bad(Move func()) {
<panel modal="false" onBack={ Move }><button onClick={ Move }>Forward</button></panel>
}`)); err == nil {
		t.Fatal("nonmodal HUD captured semantic Back")
	}
}
