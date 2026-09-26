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

	view, err := Compile("inventory.ui", []byte(`kartui Inventory(Title string, Enabled bool, Use func()) {
<panel modal="true">
 <label>{ Title + " {items}" }</label>
 <button enabled={ Enabled } onClick={ Use }>Use &amp; refresh</button>
</panel> }`))
	if err != nil {
		t.Fatal(err)
	}

	if view.Name != "Inventory" || len(view.Parameters) != 3 || view.Bindings[0].Text != `Title + " {items}"` ||
		view.Template.Elements[1].Text != "Use & refresh" ||
		view.Template.Elements[1].Action != 2 {
		t.Fatalf("bad lowering: %+v", view)
	}
}

func TestRejectUnsupportedSource(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`kartui Menu() { <panel><button>Missing handler</button></panel> }`,
		`kartui Menu() { <panel><label style="red">x</label></panel> }`,
		`kartui Menu() { <panel><label><panel/></label></panel> }`,
		`kartui Menu(Title int) { <panel><label>x</label></panel> }`,
		`kartui Menu(Title string, Title bool) { <panel><label>x</label></panel> }`,
		`kartui Menu() { <panel><label>{ + }</label></panel> }`,
		`kartui Menu() { <panel><label>{ title </label></panel> }`,
		`kartui Menu() { <panel><label>Hello { title }</label></panel> }`,
		`kartui Menu() { <panel><label rows={ Rows }>x</label></panel> }`,
		`kartui Menu() { <!DOCTYPE panel><panel><label>x</label></panel> }`,
		`kartui Menu() { <panel><label>x</label></panel><panel/> }`,
		`kartui Menu() { <panel custom="true"><label>x</label></panel> }`,
	} {
		if _, err := Compile("broken.ui", []byte(source)); err == nil {
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

	layout := `layout WindowLayout {
<panel modal="true" class="screen"><panel class="window"><slot name="title"><label>Untitled</label></slot><panel class="content"><slot/></panel></panel></panel>
}
style {
.screen { background: theme.colors.surface; padding: 8; transition-duration: 180; }
.window { padding: 12; gap: 6; max-width: 600; }
.content { padding: 4; gap: 3; flex-direction: row; align-items: center; justify-content: space-between; overflow: scroll; }
}`
	view := `kartui Menu(Title string, Play func()) {
<WindowLayout><fragment slot="title"><label class="title">{ Title }</label></fragment><button class="primary" onClick={ Play }>Play</button></WindowLayout>
}
style {
.title { color: theme.colors.accent; }
.primary { background: theme.colors.primary; color: theme.colors.text; }
.primary:hover { background: theme.colors.primary-hover; color: theme.colors.accent; }
.primary:pressed { background: theme.colors.primary-pressed; color: theme.colors.text; }
.primary:disabled { background: #111111ff; color: #777777ff; }
}`

	if err := os.WriteFile(filepath.Join(directory, "ui", "layouts", "window.ui"), []byte(layout), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(directory, "ui", "views", "menu.ui"), []byte(view), 0o600); err != nil {
		t.Fatal(err)
	}

	components, err := LoadProjectWithTheme(directory,
		[]Source{{Name: "ui.menu", Source: "ui/views/menu.ui"}},
		[]LayoutSource{{Source: "ui/layouts/window.ui"}}, "")
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

	layout, err := parseLayout("window.ui", []byte(`layout WindowLayout { <panel><slot/></panel> }`))
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = expandLayouts(
		"menu.ui",
		`<WindowLayout><fragment slot="missing"><label>x</label></fragment></WindowLayout>`,
		map[string]Layout{"WindowLayout": layout},
	)
	if err == nil {
		t.Fatal("accepted an unknown named slot")
	}
}

func TestLayoutAlwaysSelectsSchemaFive(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("plain.ui", []byte(`layout PlainLayout { <panel modal="true"><slot/></panel> }`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := compileWithLayouts(
		"menu.ui",
		[]byte(`kartui Menu() { <PlainLayout><label>Menu</label></PlainLayout> }`),
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

	if _, err := DecodeSource("hud.ui", []byte(`kartui Hud() { <panel><label>My level</label></panel> }`)); err != nil {
		t.Fatal(err)
	}

	if _, err := DecodeSource("hud.ui", []byte(`kartui Hud(Title string) { <panel><label>{ Title }</label></panel> }`)); err == nil {
		t.Fatal("silently dropped dynamic level binding")
	}
}

func TestLocalUISetup(t *testing.T) {
	t.Parallel()

	source := `import "strconv"
kartui Inventory(game *Game) {
 setup { message := "}"; count := 0; click := func() { count++; message = strconv.Itoa(count); invalidate() } }
 <panel modal="true"><label>{ message }</label><button onClick={ click }>Click</button></panel>
}`

	view, err := Compile("inventory.ui", []byte(source))
	if err != nil {
		t.Fatal(err)
	}

	if !view.Local || !strings.Contains(view.Setup, "count++") || !strings.Contains(view.Preamble, "strconv") {
		t.Fatalf("missing local Go: %+v", view)
	}

	if _, err := DecodeSource("inventory.ui", []byte(source)); err == nil {
		t.Fatal("executed client code in passive level asset")
	}
}

func TestLocalUIProps(t *testing.T) {
	t.Parallel()

	view, err := Compile("inventory.ui", []byte(`type InventoryProps struct { Title string; Use func() }
kartui Inventory(props InventoryProps) { <panel modal="true"><label>{ props.Title }</label><button onClick={ props.Use }>Use</button></panel> }`))
	if err != nil {
		t.Fatal(err)
	}

	if !view.Local || !strings.Contains(view.Preamble, "type InventoryProps struct") {
		t.Fatalf("missing portable props: %+v", view)
	}
}

func TestSeparateSetup(t *testing.T) {
	t.Parallel()

	view, err := Compile("menu.ui", []byte(`type Props struct { setup string }
// setup { ignored in comment
setup {
    message := "}"
    click := func() { message = "clicked"; invalidate() }
}
kartui Menu(props Props) { <panel modal="true"><label>{ message }</label><button onClick={ click }>Click</button></panel> }`))
	if err != nil || !view.Local || !strings.Contains(view.Setup, "invalidate()") || strings.Contains(view.Preamble, "message :=") {
		t.Fatalf("separate setup: %+v, %v", view, err)
	}

	for _, source := range []string{
		"setup {} setup {}\nkartui Menu() { <panel/> }",
		"setup {}\nkartui Menu() { setup {} <panel/> }",
		"setup {\nkartui Menu() { <panel/> }",
		"setup nope\nkartui Menu() { <panel/> }",
	} {
		if _, err := Compile("bad.ui", []byte(source)); err == nil {
			t.Fatalf("accepted invalid setup: %s", source)
		}
	}
}

func TestNamedSetup(t *testing.T) {
	t.Parallel()

	view, err := Compile("menu.ui", []byte(`setup Menu(props Props) {
 title := props.Title
}
kartui Menu { <panel><label>{ title }</label></panel> }`))
	if err != nil || view.Name != "Menu" || !view.Local || len(view.Parameters) != 1 || view.Parameters[0].Name != "props" {
		t.Fatalf("named setup: %+v, %v", view, err)
	}

	for _, source := range []string{
		"setup Menu(props Props) {}\nkartui Other { <panel/> }",
		"setup Menu(props Props) {}\nkartui Menu(props Props) { <panel/> }",
		"setup Menu(props Props) {}\nkartui Menu { setup {} <panel/> }",
		"setup Menu {}\nkartui Menu { <panel/> }",
		"setup Menu() {} setup Menu() {}\nkartui Menu { <panel/> }",
		"kartui Menu { <panel/> }",
	} {
		if _, err := Compile("bad.ui", []byte(source)); err == nil {
			t.Fatalf("accepted invalid named setup: %s", source)
		}
	}
}

func TestProjectConfinementAndDuplicateComponents(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	contents := []byte(`kartui Menu() { <panel><label>Menu</label></panel> }`)
	if err := os.WriteFile(filepath.Join(root, "menu.ui"), contents, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := Load(root, []Source{{Name: "ui.one", Source: "menu.ui"}, {Name: "ui.two", Source: "menu.ui"}}); err == nil {
		t.Fatal("duplicate component accepted")
	}

	if _, err := ReadSource(root, "../outside.ui"); err == nil {
		t.Fatal("escaped project")
	}

	if _, err := Compile("large.ui", []byte(strings.Repeat("x", 65537))); err == nil {
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

	component, err := CompileWithTheme("menu.ui", []byte(`kartui Menu() {
<panel modal="true" class="screen"><label class="title">Menu</label><button class="primary" onClick={ Play }>Play</button></panel>
}

style {
.screen { background: theme.colors.surface; padding: theme.spacing.panel; }
.title { color: theme.colors.brand; font-size: theme.typography.title; }
.primary { background: theme.colors.primary; min-height: theme.controls.height; }
}`), theme)
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

	component, err := Compile("menu.ui", []byte(`kartui Menu() {
<panel modal="true" class="screen"><label class="title">Menu</label><button class="primary" onClick={ Play }>Play</button></panel>
}

style {
.screen { padding: 24; }
.title { font-size: 30; }
.primary { min-height: 48; }
@media (max-width: 480) {
    .screen { padding: 8; }
    label { font-size: 16; }
    .title { font-size: 24; max-width: 300; }
    button { font-size: 16; min-height: 40; }
}
}`))
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

	component, err := Compile("menu.ui", []byte(`kartui Menu() {
<panel modal="true"><panel class="drawer"><button class="primary" onClick={ Play }>Play</button></panel></panel>
}

style {
.drawer {
    transition-duration: 180;
    transition-enter: slide-from-left;
    transition-exit: slide-to-right;
}
.primary {
    transition-duration: 140;
    background: #203040;
}
}`))
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

	component, err := Compile("menu.ui", []byte(`kartui Menu() {
<panel modal="true"><label class="status">Ready</label><button class="action" onClick={ Play }>Play</button></panel>
}
style {
.status { text-align: right; font-family: mono; }
.action { text-align: left; font-family: display; }
}`))
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

	component, err := Compile("menu.ui", []byte(`kartui Menu() {
<panel modal="true"><button class="action" onClick={ Play }>Play</button></panel>
}
style {
.action { transition-duration: 180; transition-delay: 40; transition-easing: ease-in-out; }
.action:hover { background: #204060; color: #ffffff; }
.action:focus { background: #70d6ff; color: #06101b; }
}`))
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

	component, err := Compile("badge.ui", []byte(`kartui Badge() {
<panel><panel class="badge"><label>Online</label></panel><label>Content</label></panel>
}
style { .badge { position: absolute; top: 12; right: 16; } }`))
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

func TestCompileRejectsInvalidStyles(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`kartui Bad() { <panel class="missing"><label>x</label></panel> }`,
		`kartui Bad() { <panel><label>x</label></panel> } style { .unused { color: #ffffff; } }`,
		`kartui Bad() { <panel><label class="x">x</label></panel> } style { .x { padding: 4; } }`,
		`kartui Bad() { <panel><label class="x">x</label></panel> } style { .x { color: theme.colors.missing; } }`,
		`kartui Bad() { <panel class="x"><label>x</label></panel> } style { .x { flex-direction: diagonal; } }`,
		`kartui Bad() { <panel><label class="x">x</label></panel> } style { .x { align-items: center; } }`,
		`kartui Bad() { <panel class="x"><label>x</label></panel> } style { .x { justify-content: around; } }`,
		`kartui Bad() { <panel class="x"><label>x</label></panel> } style { .x { overflow: clip; } }`,
		`kartui Bad() { <panel><label class="x">x</label></panel> } style { .x { text-align: justify; } }`,
		`kartui Bad() { <panel class="x"><label>x</label></panel> } style { .x { text-align: left; } }`,
		`kartui Bad() { <panel><label class="x">x</label></panel> } style { .x { font-family: fantasy; } }`,
		`kartui Bad() { <panel modal="true"><button class="x" onClick={ Play }>x</button></panel> } style { .x { transition-easing: bounce; } }`,
		`kartui Bad() { <panel><panel class="x"><label>x</label></panel></panel> } style { .x { position: fixed; } }`,
		`kartui Bad() { <panel><panel class="x"><label>x</label></panel></panel> } style { .x { top: 10; } }`,
		`kartui Bad() { <panel><label class="x">x</label></panel> } style { .x { flex-grow: 17; } }`,
		`kartui Bad() { <panel><label class="x">x</label></panel> } style { .x { margin: 257; } }`,
		`kartui Bad() { <panel><label>x</label></panel> } style { @media (max-width: 200) { label { font-size: 12; } } }`,
		`kartui Bad() { <panel><label>x</label></panel> } style { @media (max-width: 480) { label { font-size: 12; } } @media (max-width: 600) { label { font-size: 14; } } }`,
		`kartui Bad() { <panel><label>x</label></panel> } style { @media (max-width: 480) { @media (max-width: 400) { label { font-size: 12; } } } }`,
	} {
		if _, err := Compile("bad.ui", []byte(source)); err == nil {
			t.Fatalf("accepted invalid style: %s", source)
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

	component, err := CompileWithTheme("menu.ui", []byte(`kartui Menu() {
<panel class="screen"><label>Menu</label></panel>
}

style { .screen { background-image: theme.images.panel; } }`), theme)
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

	component, err := CompileWithTheme("menu.ui", []byte(`kartui Menu() {
<panel><image class="emblem"/></panel>
}
style { .emblem { image: theme.images.emblem; min-width: 48; min-height: 48; } }`), theme)
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

	component, err := CompileWithTheme("menu.ui", []byte(`kartui Menu(Click func()) {
<panel modal="true"><image class="art"/><button class="play" onClick={ Click }>Play</button></panel>
}
style {
 .art { image: theme.images.emblem; image-fit: cover; tint: #70d6ffff; min-width: 96; min-height: 48; }
 .play { icon: theme.images.emblem; icon-size: 18; icon-gap: 7; icon-position: start; tint: #ffffffff; }
}`), theme)
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

	layout, err := parseLayout("window.ui", []byte(`layout WindowLayout {
<panel modal="true"><slot/></panel>
}`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := compileWithLayouts("inventory.ui", []byte(`kartui Inventory(Back func()) {
<WindowLayout onBack={ Back }><Child props={ Back }/></WindowLayout>
}`), DefaultTheme(), map[string]Layout{"WindowLayout": layout})
	if err != nil {
		t.Fatal(err)
	}

	if !component.Template.Back || component.Back != "Back" || component.Template.Version != ui.SchemaInteractionPolish {
		t.Fatalf("semantic back = template %+v callback %q", component.Template, component.Back)
	}
}
