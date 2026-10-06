package uicompiler

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	ui "github.com/karty-game/karty-ui/schema"
)

func TestIndentedStyleVariablesStatesSizingAndResponsiveAuto(t *testing.T) {
	t.Parallel()

	source := `<template><panel class="screen"><button class="primary" onClick={ func() {} }>Play</button></panel></template>
<style lang="sass">
$space: theme.spacing.gap
$alias: $space
.screen
  width: 80%
  height: 240px
  direction: row
  align: stretch
  justify: center
  gap: $alias
.primary
  width: 33.33%
  height: 44px
  grow: 1
  &:hover
    background: theme.colors.primary-hover
@media (max-width: 480)
  .primary
    width: auto
</style>`

	component, err := Compile("menu.kui", []byte(source))
	if err != nil {
		t.Fatal(err)
	}

	panel, button := component.Template.Panel, component.Template.Elements[0]
	if component.Template.Version != ui.SchemaSizing || panel.Width.Value != 8000 || panel.Height.Value != 240 || panel.Gap != 8 ||
		panel.FlexDirection != ui.FlexDirectionRow {
		t.Fatalf("unexpected panel: %+v", panel)
	}

	if button.Style.Width.Value != 3333 || button.Style.Height.Value != 44 || button.Style.FlexGrow != 1 ||
		button.Style.Set&ui.StyleBackgroundHover == 0 {
		t.Fatalf("unexpected button: %+v", button.Style)
	}

	if button.ResponsiveStyle.Width != (ui.Length{}) || button.ResponsiveStyle.Set2&ui.Style2Width == 0 {
		t.Fatalf("auto override was lost: %+v", button.ResponsiveStyle)
	}
	// The compact default style tag accepts the same indentation syntax.
	if _, err := Compile("menu.kui", []byte(strings.Replace(source, `<style lang="sass">`, `<style>`, 1))); err != nil {
		t.Fatal(err)
	}
}

func TestIndentedStylesRejectAmbiguityAndUnboundedExpansion(t *testing.T) {
	t.Parallel()

	cases := []string{
		".screen\n\twidth: 50%", ".screen\n  width: 50%;",
		".screen\n  width: 50%\n height: 20px",
		".screen\n  &:hover\n    &:focus\n      &:pressed\n        color: #ffffff",
		"$big: " + strings.Repeat("x", 4096) + "\n.screen\n" + strings.Repeat("  color: $big\n", 32),
	}
	for _, styles := range cases {
		_, err := Compile(
			"bad.kui",
			[]byte("<template><panel class=\"screen\"><label>Bad</label></panel></template>\n<style lang=\"sass\">\n"+styles+"\n</style>"),
		)
		if !errors.Is(err, ui.ErrTemplate) {
			t.Fatalf("accepted invalid style %q: %v", styles, err)
		}
	}
}

func TestStyleDimensionsAndPixelUnits(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"0%", "100%", "0.01%", "33.33%", "0px", "2048px", "64", "auto"} {
		source := "kartui Size() { <panel><label>Size</label></panel> }\nstyle { panel { width: " + value + "; padding: 8px; gap: 4px; } }"

		component, err := Compile("size.kui", []byte(source))
		if err != nil || component.Template.Version != ui.SchemaSizing {
			t.Fatalf("dimension %s: %v", value, err)
		}
	}

	for _, value := range []string{"-1%", "+1%", "100.01%", "101%", "1.001%", "1.%", "NaN%", "2049px", "1em", "1.5px"} {
		component, err := Compile(
			"size.kui",
			[]byte("kartui Size() { <panel><label>Size</label></panel> }\nstyle { panel { width: "+value+"; } }"),
		)
		if err != nil || len(component.StyleWarnings()) == 0 || component.Template.Panel.Set2&ui.Style2Width != 0 {
			t.Fatalf("invalid dimension should warn and be omitted: %s: %v", value, err)
		}
	}
}

func TestIndentedLayoutStylesResolveWithinProjectedComponent(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("frame.kui", []byte(`<template><panel class="frame"><slot/></panel></template>
<style>
$width: 90%
.frame
  width: $width
  padding: 12px
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := compileWithLayouts(
		"page.kui",
		[]byte(`<template><Frame><label>Content</label></Frame></template>`),
		DefaultTheme(),
		map[string]Layout{"Frame": layout},
	)
	if err != nil || component.Template.Version != ui.SchemaSizing || component.Template.Panel.Width.Value != 9000 {
		t.Fatalf("layout sizing: %+v %v", component.Template, err)
	}
}

func TestIndentedStyleReferenceExampleCompiles(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("../docs/0.0.1/indented-styles.md")
	if err != nil {
		t.Fatal(err)
	}

	_, example, found := strings.Cut(string(contents), "```kui\n")
	if !found {
		t.Fatal("missing indented style example")
	}

	example, _, found = strings.Cut(example, "```")
	if !found {
		t.Fatal("unclosed example")
	}

	if _, err := Compile("menu.kui", []byte(example)); err != nil {
		t.Fatal(err)
	}
}

func TestIndentedStylesBoundVariableExpansion(t *testing.T) {
	t.Parallel()

	var source strings.Builder
	source.WriteString("$long: " + strings.Repeat("x", 4096) + "\n.screen\n")

	for index := range 32 {
		fmt.Fprintf(&source, "  value-%d: $long\n", index)
	}

	_, err := lowerIndentedStyles("large.kui", source.String())
	if !errors.Is(err, ui.ErrTemplate) || !strings.Contains(err.Error(), "expanded styles") {
		t.Fatalf("unbounded variable expansion: %v", err)
	}
}

func TestIndentedStylesBoundNestedAndMediaVariableExpansion(t *testing.T) {
	t.Parallel()

	for _, media := range []bool{false, true} {
		var source strings.Builder
		source.WriteString("$long: " + strings.Repeat("x", 4096) + "\n")

		if media {
			source.WriteString("@media (max-width: 480)\n")
		}

		indent := ""
		if media {
			indent = "  "
		}

		for index := range 32 {
			if media {
				source.WriteString(indent + ".screen\n" + indent + "  color: $long\n")
			} else {
				if index == 0 {
					source.WriteString(".screen\n")
				}

				source.WriteString("  &:hover\n    color: $long\n")
			}
		}

		_, err := lowerIndentedStyles("large.kui", source.String())
		if !errors.Is(err, ui.ErrTemplate) || !strings.Contains(err.Error(), "expanded styles") {
			t.Fatalf("unbounded media=%t expansion: %v", media, err)
		}
	}
}
