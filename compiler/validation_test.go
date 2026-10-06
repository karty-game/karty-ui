package uicompiler

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	ui "github.com/karty-game/karty-ui/schema"
)

func TestMixedFeatureSchemaIndependentOfOrder(t *testing.T) {
	t.Parallel()

	theme := DefaultTheme()
	theme.Images["portrait"] = ui.Image{Scope: ui.ImageScopeGame, Name: "portrait"}

	for _, test := range []struct {
		name, first, second, rootStyle, styles string
		version                                uint32
	}{
		{"text and focus", `<label class="title">Title</label>`, `<button class="focus" onClick={Click}>Play</button>`, "",
			`.title { text-align: center; } .focus:focus { color: #ffffffff; }`, ui.SchemaInteractionPolish},
		{"image and focus", `<image class="portrait"/>`, `<button class="focus" onClick={Click}>Play</button>`, "",
			`.portrait { image: theme.images.portrait; } .focus:focus { color: #ffffffff; }`, ui.SchemaInteractionPolish},
		{"transition and text", `<button class="animated" onClick={Click}>Play</button>`, `<label class="title">Title</label>`, "",
			`.animated { transition-duration: 100; } .title { text-align: center; }`, ui.SchemaVisualHierarchy},
		{"root transition and focus", `<label>Title</label>`, `<button class="focus" onClick={Click}>Play</button>`,
			`panel { transition-duration: 100; transition-enter: slide-from-left; }`,
			`.focus:focus { color: #ffffffff; }`, ui.SchemaInteractionPolish},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, markup := range []string{test.first + test.second, test.second + test.first} {
				source := "kartui Menu(Click func()) { <panel modal=\"true\">" + markup + "</panel> }\nstyle {" + test.rootStyle + test.styles + "}"

				component, err := CompileWithTheme("menu.ui", []byte(source), theme)
				if err != nil {
					t.Fatal(err)
				}

				if component.Template.Version != test.version {
					t.Fatalf("schema=%d, want %d", component.Template.Version, test.version)
				}
			}
		})
	}
}

func TestStaticLevelRejectsConditionalVisibility(t *testing.T) {
	t.Parallel()

	for _, condition := range []string{"false", "Shown"} {
		markup := fmt.Sprintf("<panel>\nif %s {\n<label>Hidden</label>\n} else {\n<label>Shown</label>\n}\n</panel>", condition)
		for source, text := range map[string]string{
			"hud.ui":  "kartui HUD(Shown bool) { " + markup + " }",
			"hud.kui": "<template>" + markup + "</template>",
		} {
			if _, err := DecodeSource(source, []byte(text)); !errors.Is(err, ui.ErrTemplate) {
				t.Fatalf("%s accepted visibility condition %q: %v", source, condition, err)
			}
		}
	}
}

func TestTextRequiresOpenControl(t *testing.T) {
	t.Parallel()

	for _, markup := range []string{
		`<panel><label/>outside</panel>`,
		`<panel><label/></panel>outside`,
		`<panel><panel><label/></panel>outside</panel>`,
		`<panel>outside<label/></panel>`,
	} {
		if _, err := Compile("menu.ui", []byte("kartui Menu() { "+markup+" }")); !errors.Is(err, ui.ErrTemplate) {
			t.Fatalf("accepted stray text %q: %v", markup, err)
		}
	}

	component, err := Compile("menu.ui", []byte("kartui Menu() { \n<panel>\n<label>Tom &amp; Jerry</label>\n<label/>\n</panel>\n }"))
	if err != nil || component.Template.Elements[0].Text != "Tom & Jerry" {
		t.Fatalf("valid text or whitespace rejected: %v", err)
	}
}

func TestElementLimitsDuringLowering(t *testing.T) {
	t.Parallel()

	for _, control := range []string{`<label/>`, `<Child props={true}/>`} {
		for _, count := range []int{ui.MaxElements, ui.MaxElements + 1} {
			source := "kartui Menu() { <panel>" + strings.Repeat(control, count) + "</panel> }"

			component, err := Compile("menu.ui", []byte(source))
			if count == ui.MaxElements && err != nil {
				t.Fatal(err)
			}

			if count > ui.MaxElements && (!errors.Is(err, ui.ErrTemplate) || len(component.Bindings) > ui.MaxElements) {
				t.Fatalf("lowering exceeded element budget: count=%d err=%v", len(component.Bindings), err)
			}
		}
	}
}
