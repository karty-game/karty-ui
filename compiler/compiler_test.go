package uicompiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/karty-game/karty-ui/schema"
)

func TestCompile(t *testing.T) {
	t.Parallel()

	view, err := Compile("inventory.kui", []byte(`<template>
<panel modal="true">
 <label>{ Title + " {items}" }</label>
 <button enabled={ Enabled } onClick={ Use }>Use &amp; refresh</button>
</panel>
</template>

<script setup lang="go">
func setup(Title string, Enabled bool, Use func()) {}
</script>`))
	if err != nil {
		t.Fatal(err)
	}

	if view.Name != "Inventory" || len(view.Parameters) != 3 || view.Bindings[0].Text != `Title + " {items}"` ||
		view.Template.Elements[1].Text != "Use & refresh" ||
		view.Template.Elements[1].Action != 2 {
		t.Fatalf("bad lowering: %+v", view)
	}
}

func TestCompileSingleFileComponentUsesFileName(t *testing.T) {
	t.Parallel()

	source := `<template>
<panel modal="true"><label class="title">{ args.Title }</label><button onClick={ choose }>Play</button></panel>
</template>

<script setup lang="go">
import "strings"

type MenuProps struct { Title string; Select func() }

func setup(args MenuProps) {
	choose := func() { _ = strings.TrimSpace(args.Title); args.Select() }
}
</script>

<style>
.title
  color: theme.colors.text
</style>`

	component, err := Compile("ui/views/menu.kui", []byte(source))
	if err != nil {
		t.Fatal(err)
	}

	if component.Name != "Menu" || !component.Local || len(component.Parameters) != 1 ||
		component.Parameters[0] != (Parameter{Name: "args", Type: "MenuProps"}) {
		t.Fatalf("component identity and parameters = %+v", component)
	}

	if !strings.Contains(component.Setup, "choose := func()") || !strings.Contains(component.Preamble, `import "strings"`) ||
		component.Bindings[0].Text != "args.Title" || component.Bindings[1].Click != "choose" {
		t.Fatalf("SFC lowering lost Go or template content: %+v", component)
	}
}

func TestCompileSFCExampleProject(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "samples", "sfc-demo")

	components, err := LoadProjectWithTheme(root,
		[]Source{{Name: "ui.menu", Source: "ui/views/menu.kui"}},
		[]LayoutSource{{Source: "ui/layouts/main-menu.kui"}}, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(components) != 1 || components[0].Name != "Menu" || len(components[0].Parameters) != 1 ||
		components[0].Parameters[0].Type != "MenuProps" ||
		components[0].Bindings[2].Click != "choose" {
		t.Fatalf("compiled SFC example lost the derived API or handler")
	}
}

func TestParseSingleFileLayoutUsesFileName(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("ui/layouts/main-menu.kui", []byte(`<template><panel modal="true"><slot/></panel></template>
<style>
.screen
  padding: 8
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	if layout.Name != "MainMenu" || !strings.Contains(layout.Style, ".screen") {
		t.Fatalf("SFC layout = %+v", layout)
	}
}

func TestRejectUnsupportedSource(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`<template>
<panel><button>Missing handler</button></panel>
</template>`,
		`<template>
<panel><label style="red">x</label></panel>
</template>`,
		`<template>
<panel><label><panel/></label></panel>
</template>`,
		`<template>
<panel><label>x</label></panel>
</template>

<script setup lang="go">
func setup(invalidate int) {}
</script>`,
		`<template>
<panel><label>x</label></panel>
</template>

<script setup lang="go">
func setup(Title string, Title bool) {}
</script>`,
		`<template>
<panel><label>{ + }</label></panel>
</template>`,
		`<template><panel><label>{ title </label></panel></template>`,
		`<template>
<panel><label>Hello { title }</label></panel>
</template>`,
		`<template>
<panel><label rows={ Rows }>x</label></panel>
</template>`,
		`<template>
<!DOCTYPE panel><panel><label>x</label></panel>
</template>`,
		`<template>
<panel><label>x</label></panel><panel/>
</template>`,
		`<template>
<panel custom="true"><label>x</label></panel>
</template>`,
	} {
		if _, err := Compile("broken.kui", []byte(source)); err == nil {
			t.Errorf("accepted unsupported source: %s", source)
		}
	}
}

func TestCompileLayoutAndNestedPanels(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, "ui", "layouts"), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(directory, "ui", "views"), 0o700); err != nil {
		t.Fatal(err)
	}

	layout := `<template>
<panel modal="true" class="screen"><panel class="window"><slot name="title"><label>Untitled</label></slot><panel class="content"><slot/></panel></panel></panel>
</template>

<style>
.screen
  background: theme.colors.surface
  padding: 8
  transition-duration: 180
.window
  padding: 12
  gap: 6
  max-width: 600
.content
  padding: 4
  gap: 3
  flex-direction: row
  align-items: center
  justify-content: space-between
  overflow: scroll
</style>`
	view := `<template>
<WindowLayout><fragment slot="title"><label class="title">{ Title }</label></fragment><button class="primary" onClick={ Play }>Play</button></WindowLayout>
</template>

<script setup lang="go">
func setup(Title string, Play func()) {}
</script>

<style>
.title
  color: theme.colors.accent
.primary
  background: theme.colors.primary
  color: theme.colors.text
.primary:hover
  background: theme.colors.primary-hover
  color: theme.colors.accent
.primary:pressed
  background: theme.colors.primary-pressed
  color: theme.colors.text
.primary:disabled
  background: #111111ff
  color: #777777ff
</style>`

	if err := os.WriteFile(filepath.Join(directory, "ui", "layouts", "window-layout.kui"), []byte(layout), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "ui", "views", "menu.kui"), []byte(view), 0o600); err != nil {
		t.Fatal(err)
	}

	components, err := LoadProjectWithTheme(directory,
		[]Source{{Name: "ui.menu", Source: "ui/views/menu.kui"}},
		[]LayoutSource{{Source: "ui/layouts/window-layout.kui"}}, "")
	if err != nil {
		t.Fatal(err)
	}

	component := components[0]
	if component.Template.Version != ui.SchemaLayout || len(component.Template.Elements) != 4 {
		t.Fatalf("layout lowering = %+v", component.Template)
	}

	if panel := component.Template.Elements[0]; panel.Kind != "panel" || panel.Parent != 0 {
		t.Fatalf("window panel = %+v", panel)
	} else if panel.Style.MaxWidth != 600 || panel.Style.Set&ui.StyleMaxWidth == 0 {
		t.Fatalf("window sizing = %+v", panel.Style)
	}

	if component.Template.Panel.TransitionDuration != 180 {
		t.Fatalf("root transition = %+v", component.Template.Panel)
	}

	if content := component.Template.Elements[2]; content.Kind != "panel" || content.Parent != component.Template.Elements[0].ID {
		t.Fatalf("content panel = %+v", content)
	} else if content.Style.FlexDirection != ui.FlexDirectionRow || content.Style.AlignItems != ui.AlignItemsCenter ||
		content.Style.JustifyContent != ui.JustifyContentSpaceBetween || content.Style.Overflow != ui.OverflowScroll {
		t.Fatalf("content layout = %+v", content.Style)
	}

	button := component.Template.Elements[3]

	const interaction = ui.StyleBackgroundDisabled | ui.StyleColorHover | ui.StyleColorPressed | ui.StyleColorDisabled
	if button.Parent != component.Template.Elements[2].ID || button.Style.Set&interaction != interaction {
		t.Fatalf("projected button = %+v", button)
	}
}

func TestLayoutRejectsUnknownSlot(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("window-layout.kui", []byte(`<template>
<panel><slot/></panel>
</template>`))
	if err != nil {
		t.Fatal(err)
	}

	_, _, origins, err := expandLayouts(
		"menu.kui",
		`<WindowLayout><fragment slot="missing"><label>x</label></fragment></WindowLayout>`, styleSource{},
		map[string]Layout{"WindowLayout": layout},
	)
	if err == nil || len(origins) != 0 {
		t.Fatal("accepted an unknown named slot")
	}
}

func TestLayoutAlwaysSelectsSchemaFive(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("plain-layout.kui", []byte(`<template>
<panel modal="true"><slot/></panel>
</template>`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := compileWithLayouts(
		"menu.kui",
		[]byte(`<template>
<PlainLayout><label>Menu</label></PlainLayout>
</template>`),
		DefaultTheme(),
		map[string]Layout{"PlainLayout": layout},
	)
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Version != ui.SchemaLayout {
		t.Fatalf("layout schema = %d, want %d", component.Template.Version, ui.SchemaLayout)
	}
}

func TestLevelStaticAndDynamic(t *testing.T) {
	t.Parallel()

	if _, err := DecodeSource("hud.kui", []byte(`<template>
<panel><label>My level</label></panel>
</template>`)); err != nil {
		t.Fatal(err)
	}

	kuiSource := []byte(`<template><panel><label>My level</label></panel></template>`)
	if _, err := DecodeSource("hud.kui", kuiSource); err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeSource("hud.kui", []byte(`<template>
<panel><label>{ Title }</label></panel>
</template>

<script setup lang="go">
func setup(Title string) {}
</script>`)); err == nil {
		t.Fatal("silently dropped dynamic level binding")
	}
}

func TestLocalUISetup(t *testing.T) {
	t.Parallel()

	source := `<template>
<panel modal="true"><label>{ message }</label><button onClick={ click }>Click</button></panel>
</template>

<script setup lang="go">
import "strconv"

func setup(game *Game) {
message := "}"; count := 0; click := func() { count++; message = strconv.Itoa(count); invalidate() }
}
</script>`

	view, err := Compile("inventory.kui", []byte(source))
	if err != nil {
		t.Fatal(err)
	}

	if !view.Local || !strings.Contains(view.Setup, "count++") || !strings.Contains(view.Preamble, "strconv") {
		t.Fatalf("missing local Go: %+v", view)
	}

	if _, err := DecodeSource("inventory.kui", []byte(source)); err == nil {
		t.Fatal("executed client code in passive level asset")
	}
}

func TestLocalUIProps(t *testing.T) {
	t.Parallel()

	view, err := Compile("inventory.kui", []byte(`<template>
<panel modal="true"><label>{ props.Title }</label><button onClick={ props.Use }>Use</button></panel>
</template>

<script setup lang="go">
type InventoryProps struct { Title string; Use func() }

func setup(props InventoryProps) {}
</script>`))
	if err != nil {
		t.Fatal(err)
	}

	if !view.Local || !strings.Contains(view.Preamble, "type InventoryProps struct") {
		t.Fatalf("missing portable props: %+v", view)
	}
}

func TestGoSetupPreservesQuotedBraces(t *testing.T) {
	t.Parallel()

	view, err := Compile("menu.kui", []byte(`<template><panel><label>{message}</label></panel></template>
<script setup lang="go">
type Props struct { setup string }
// setup { ignored in comment
func setup(props Props) {
 message := "}"
 _ = props
}
</script>`))
	if err != nil || !view.Local || !strings.Contains(view.Setup, `message := "}"`) || !strings.Contains(view.Preamble, "type Props") {
		t.Fatalf("Go setup: %+v %v", view, err)
	}
}

func TestRejectLegacyDeclarations(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		"kartui Menu() { <panel/> }",
		"setup Menu() {}\nkartui Menu { <panel/> }",
		"setup {}\nkartui Menu() { <panel/> }",
		"layout Menu { <panel/> }",
	} {
		if _, err := Compile("menu.kui", []byte(source)); err == nil {
			t.Fatalf("accepted legacy source: %s", source)
		}

		if _, err := parseLayout("menu.kui", []byte(source)); err == nil {
			t.Fatalf("accepted legacy layout: %s", source)
		}
	}
}

func TestProjectConfinementAndDuplicateComponents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	contents := []byte(`<template>
<panel><label>Menu</label></panel>
</template>`)
	if err := os.WriteFile(filepath.Join(root, "menu.kui"), contents, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(root, []Source{{Name: "ui.one", Source: "menu.kui"}, {Name: "ui.two", Source: "menu.kui"}}); err == nil {
		t.Fatal("duplicate component accepted")
	}

	if _, err := ReadSource(root, "../outside.kui"); err == nil {
		t.Fatal("escaped project")
	}

	if _, err := Compile("large.kui", []byte(strings.Repeat("x", 65537))); err == nil {
		t.Fatal("unbounded source")
	}
}

func TestCompileStylesAndTheme(t *testing.T) {
	t.Parallel()

	theme, err := ParseTheme("theme.toml", []byte(`extends = "dark"
[colors]
brand = "#123456"
[spacing]
panel = 22
[typography]
title = 31
[controls]
height = 50
`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := CompileWithTheme("menu.kui", []byte(`<template>
<panel modal="true" class="screen"><label class="title">Menu</label><button class="primary" onClick={ Play }>Play</button></panel>
</template>

<style>
.screen
  background: theme.colors.surface
  padding: theme.spacing.panel
.title
  color: theme.colors.brand
  font-size: theme.typography.title
.primary
  background: theme.colors.primary
  min-height: theme.controls.height
</style>`), theme)
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Version != 3 || component.Template.Panel.Padding != 22 || component.Template.Elements[0].Style.FontSize != 31 ||
		component.Template.Elements[1].Style.MinHeight != 50 {
		t.Fatalf("styles not compiled: %+v", component.Template)
	}

	if component.Template.Elements[0].Style.Color != 0x123456ff {
		t.Fatalf("color = %#x", component.Template.Elements[0].Style.Color)
	}
}

func TestCompileResponsiveStyles(t *testing.T) {
	t.Parallel()

	component, err := Compile("menu.kui", []byte(`<template>
<panel modal="true" class="screen"><label class="title">Menu</label><button class="primary" onClick={ Play }>Play</button></panel>
</template>

<style>
.screen
  padding: 24
.title
  font-size: 30
.primary
  min-height: 48
@media (max-width: 480)
  .screen
    padding: 8
  label
    font-size: 16
  .title
    font-size: 24
    max-width: 300
  button
    font-size: 16
    min-height: 40
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Version != ui.SchemaResponsive || component.Template.ResponsiveMaxWidth != 480 ||
		component.Template.ResponsivePanel.Padding != 8 {
		t.Fatalf("responsive root not compiled: %+v", component.Template)
	}

	title, button := component.Template.Elements[0], component.Template.Elements[1]
	if title.Style.FontSize != 30 || title.ResponsiveStyle.FontSize != 24 || title.ResponsiveStyle.MaxWidth != 300 ||
		button.Style.MinHeight != 48 || button.ResponsiveStyle.FontSize != 16 || button.ResponsiveStyle.MinHeight != 40 {
		t.Fatalf("responsive elements not compiled: %+v %+v", title, button)
	}
}

func TestCompileControlTransitions(t *testing.T) {
	t.Parallel()

	component, err := Compile("menu.kui", []byte(`<template>
<panel modal="true"><panel class="drawer"><button class="primary" onClick={ Play }>Play</button></panel></panel>
</template>

<style>
.drawer
  transition-duration: 180
  transition-enter: slide-from-left
  transition-exit: slide-to-right
.primary
  transition-duration: 140
  background: #203040
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	drawer, style := component.Template.Elements[0].Style, component.Template.Elements[1].Style
	if component.Template.Version != ui.SchemaControlTransition ||
		style.Set&ui.StyleTransitionDuration == 0 || style.TransitionDuration != 140 ||
		drawer.TransitionEnter != ui.TransitionLeft || drawer.TransitionExit != ui.TransitionRight {
		t.Fatalf("transitions not compiled: version=%d drawer=%+v button=%+v", component.Template.Version, drawer, style)
	}
}

func TestCompileTextAlignment(t *testing.T) {
	t.Parallel()

	component, err := Compile("menu.kui", []byte(`<template>
<panel modal="true"><label class="status">Ready</label><button class="action" onClick={ Play }>Play</button></panel>
</template>

<style>
.status
  text-align: right
  font-family: mono
.action
  text-align: left
  font-family: display
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Version != ui.SchemaVisualHierarchy ||
		component.Template.Elements[0].Style.TextAlign != ui.TextAlignEnd ||
		component.Template.Elements[0].Style.FontFamily != ui.FontFamilyMono ||
		component.Template.Elements[1].Style.TextAlign != ui.TextAlignStart {
		t.Fatalf("text alignment not compiled: %+v", component.Template)
	}
}

func TestCompileDistinctFocusAndTransitionTiming(t *testing.T) {
	t.Parallel()

	component, err := Compile("menu.kui", []byte(`<template>
<panel modal="true"><button class="action" onClick={ Play }>Play</button></panel>
</template>

<style>
.action
  transition-duration: 180
  transition-delay: 40
  transition-easing: ease-in-out
.action:hover
  background: #204060
  color: #ffffff
.action:focus
  background: #70d6ff
  color: #06101b
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	style := component.Template.Elements[0].Style

	want := ui.Style2BackgroundFocus | ui.Style2ColorFocus | ui.Style2TransitionDelay | ui.Style2TransitionEasing
	if component.Template.Version != ui.SchemaInteractionPolish || style.Set2&want != want ||
		style.BackgroundFocus == style.BackgroundHover || style.TransitionDelay != 40 ||
		style.TransitionEasing != ui.TransitionEasingInOut {
		t.Fatalf("interaction polish not compiled: version=%d style=%+v", component.Template.Version, style)
	}
}

func TestCompileOverlayPosition(t *testing.T) {
	t.Parallel()

	component, err := Compile("badge.kui", []byte(`<template>
<panel><panel class="badge"><label>Online</label></panel><label>Content</label></panel>
</template>

<style>
.badge
  position: absolute
  top: 12
  right: 16
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	style := component.Template.Elements[0].Style

	want := ui.Style2Position | ui.Style2Top | ui.Style2Right
	if component.Template.Version != ui.SchemaInteractionPolish || style.Set2&want != want ||
		style.Position != ui.PositionOverlay || style.Top != 12 || style.Right != 16 {
		t.Fatalf("overlay not compiled: version=%d style=%+v", component.Template.Version, style)
	}
}

func TestCompileWarnsAndIgnoresInvalidStyles(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`<template>
<panel class="missing"><label>x</label></panel>
</template>`,
		`<template>
<panel><label>x</label></panel>
</template>

<style>
.unused
  color: #ffffff
</style>`,
		`<template>
<panel><label class="x">x</label></panel>
</template>

<style>
.x
  padding: 4
</style>`,
		`<template>
<panel><label class="x">x</label></panel>
</template>

<style>
.x
  color: theme.colors.missing
</style>`,
		`<template>
<panel class="x"><label>x</label></panel>
</template>

<style>
.x
  flex-direction: diagonal
</style>`,
		`<template>
<panel><label class="x">x</label></panel>
</template>

<style>
.x
  align-items: center
</style>`,
		`<template>
<panel class="x"><label>x</label></panel>
</template>

<style>
.x
  justify-content: around
</style>`,
		`<template>
<panel class="x"><label>x</label></panel>
</template>

<style>
.x
  overflow: clip
</style>`,
		`<template>
<panel><label class="x">x</label></panel>
</template>

<style>
.x
  text-align: justify
</style>`,
		`<template>
<panel class="x"><label>x</label></panel>
</template>

<style>
.x
  text-align: left
</style>`,
		`<template>
<panel><label class="x">x</label></panel>
</template>

<style>
.x
  font-family: fantasy
</style>`,
		`<template>
<panel modal="true"><button class="x" onClick={ Play }>x</button></panel>
</template>

<style>
.x
  transition-easing: bounce
</style>`,
		`<template>
<panel><panel class="x"><label>x</label></panel></panel>
</template>

<style>
.x
  position: fixed
</style>`,
		`<template>
<panel><panel class="x"><label>x</label></panel></panel>
</template>

<style>
.x
  top: 10
</style>`,
		`<template>
<panel><label class="x">x</label></panel>
</template>

<style>
.x
  flex-grow: 17
</style>`,
		`<template>
<panel><label class="x">x</label></panel>
</template>

<style>
.x
  margin: 257
</style>`,
		`<template>
<panel><label>x</label></panel>
</template>

<style>
@media (max-width: 200)
  label
    font-size: 12
</style>`,
		`<template>
<panel><label>x</label></panel>
</template>

<style>
@media (max-width: 480)
  label
    font-size: 12
@media (max-width: 600)
  label
    font-size: 14
</style>`,
		`<template>
<panel><label>x</label></panel>
</template>

<style>
@media (max-width: 480)
  @media (max-width: 400)
    label
      font-size: 12
</style>`,
	} {
		component, err := Compile("bad.kui", []byte(source))
		if err != nil || len(component.StyleWarnings()) == 0 {
			t.Fatalf("style should warn without failing: %s: %v warnings=%v", source, err, component.StyleWarnings())
		}
	}

	for _, theme := range []string{`extends = "neon"`, `extends = "dark"
[unknown]
value = 1`} {
		if _, err := ParseTheme("theme.toml", []byte(theme)); err == nil {
			t.Fatalf("accepted invalid theme: %s", theme)
		}
	}
}

func TestCompileThemeNineSlice(t *testing.T) {
	t.Parallel()

	theme, err := ParseTheme("theme.toml", []byte(`extends = "dark"
[images.panel]
asset = "ui.panel"
slice = [10, 12, 14, 16]
`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := CompileWithTheme("menu.kui", []byte(`<template>
<panel class="screen"><label>Menu</label></panel>
</template>

<style>
.screen
  background-image: theme.images.panel
</style>`), theme)
	if err != nil {
		t.Fatal(err)
	}

	image := component.Template.Panel.BackgroundImage
	if component.Template.Version != 4 || image.Scope != ui.ImageScopeGame || image.Name != "ui.panel" ||
		image.Top != 10 || image.Right != 12 || image.Bottom != 14 || image.Left != 16 {
		t.Fatalf("compiled image style = %+v", component.Template)
	}

	if err := theme.ValidateGameImages(map[string][2]int{"ui.panel": {64, 64}}); err != nil {
		t.Fatal(err)
	}

	if err := theme.ValidateGameImages(map[string][2]int{"ui.panel": {20, 20}}); err == nil {
		t.Fatal("accepted a slice with no stretchable center")
	}

	resolved, err := theme.ResolveLevelImages(map[string]uint32{"ui.panel": 7}, map[string][2]int{"ui.panel": {64, 64}})
	if err != nil {
		t.Fatal(err)
	}

	if image := resolved.Images["panel"]; image.Scope != ui.ImageScopeLevel || image.AssetID != 7 || image.Name != "" {
		t.Fatalf("resolved level image = %+v", image)
	}
}

func TestCompileForegroundImage(t *testing.T) {
	t.Parallel()

	theme, err := ParseTheme("theme.toml", []byte(`extends = "dark"
[images.emblem]
asset = "ui.emblem"
slice = [0, 0, 0, 0]
`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := CompileWithTheme("menu.kui", []byte(`<template>
<panel><image class="emblem"/></panel>
</template>

<style>
.emblem
  image: theme.images.emblem
  min-width: 48
  min-height: 48
</style>`), theme)
	if err != nil {
		t.Fatal(err)
	}

	style := component.Template.Elements[0].Style
	if component.Template.Version != ui.SchemaVisualHierarchy || component.Template.Elements[0].Kind != "image" ||
		style.ContentImage.Name != "ui.emblem" || style.MinWidth != 48 || style.MinHeight != 48 {
		t.Fatalf("foreground image not compiled: %+v", component.Template)
	}
}

func TestCompileImageFitTintAndButtonIcon(t *testing.T) {
	t.Parallel()

	theme, err := ParseTheme("theme.toml", []byte(`extends = "dark"
[images.emblem]
asset = "ui.emblem"
slice = [0, 0, 0, 0]
`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := CompileWithTheme("menu.kui", []byte(`<template>
<panel modal="true"><image class="art"/><button class="play" onClick={ Click }>Play</button></panel>
</template>

<script setup lang="go">
func setup(Click func()) {}
</script>

<style>
.art
  image: theme.images.emblem
  image-fit: cover
  tint: #70d6ffff
  min-width: 96
  min-height: 48
.play
  icon: theme.images.emblem
  icon-size: 18
  icon-gap: 7
  icon-position: start
  tint: #ffffffff
</style>`), theme)
	if err != nil {
		t.Fatal(err)
	}

	imageStyle := component.Template.Elements[0].Style
	buttonStyle := component.Template.Elements[1].Style

	if component.Template.Version != ui.SchemaInteractionPolish || imageStyle.ImageFit != ui.ImageFitCover ||
		imageStyle.Tint != 0x70d6ffff || buttonStyle.Icon.Name != "ui.emblem" ||
		buttonStyle.IconSize != 18 || buttonStyle.IconGap != 7 || buttonStyle.IconPosition != ui.IconPositionStart {
		t.Fatalf("image/icon styles not compiled: image=%+v button=%+v", imageStyle, buttonStyle)
	}
}

func TestCompileSemanticBackThroughLayout(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("window-layout.kui", []byte(`<template>
<panel modal="true"><slot/></panel>
</template>`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := compileWithLayouts("inventory.kui", []byte(`<template>
<WindowLayout onBack={ Back }><Child props={ Back }/></WindowLayout>
</template>

<script setup lang="go">
func setup(Back func()) {}
</script>`), DefaultTheme(), map[string]Layout{"WindowLayout": layout})
	if err != nil {
		t.Fatal(err)
	}

	if !component.Template.Back || component.Back != "Back" || component.Template.Version != ui.SchemaInteractionPolish {
		t.Fatalf("semantic back = template %+v callback %q", component.Template, component.Back)
	}
}
