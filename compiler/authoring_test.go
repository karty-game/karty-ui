package uicompiler

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	ui "github.com/karty-game/karty-ui/schema"
)

func TestSFCBlockEditingAcceptsCommentsAndAttributeOrder(t *testing.T) {
	t.Parallel()

	for _, opening := range []string{`<script lang="go" setup>`, `<script  setup  lang = 'go' >`, `<script
 lang="go"
 setup
>`} {
		component, err := Compile(
			"menu.kui",
			[]byte(
				"<!-- Menu -->\n"+opening+"\nfunc setup() { title := \"Ready\" }\n</script >\n<!-- Content -->\n<template >\n<panel><label>{title}</label></panel>\n</template >\n<!-- Appearance -->\n<style lang = 'sass' >\npanel\n  gap: 8px\n</style >\n<!-- End -->",
			),
		)
		if err != nil || component.Template.Panel.Gap != 8 || component.Bindings[0].Text != "title" {
			t.Fatalf("opening %s: %v %+v", opening, err, component)
		}
	}
}

//nolint:dupword // Duplicate attributes are intentionally malformed test input.
func TestSFCBlockEditingStillRejectsMalformedStructure(t *testing.T) {
	t.Parallel()

	for _, opening := range []string{`<script setup setup lang="go">`, `<script setup lang="go" lang="go">`, `<script setup="true" lang="go">`, `<script setup lang="go" other="x">`, `<script setup lang=go>`, `<script setup lang="go"setup>`} {
		_, err := Compile("bad.kui", []byte(opening+`func setup() {}</script><template><panel><label>Text</label></panel></template>`))

		var failure *SourceError
		if !errors.Is(err, ui.ErrTemplate) || !errors.As(err, &failure) || failure.Line != 1 {
			t.Fatalf("opening accepted or lost location: %s %v", opening, err)
		}
	}

	for _, source := range []string{`<!-- unfinished`, `<template><panel><label>Text</label></panel></template><!-- unfinished`, `<template><panel><label>Text</label></panel></template><template/>`} {
		if _, err := Compile("bad.kui", []byte(source)); err == nil {
			t.Fatal("accepted malformed SFC", source)
		}
	}
}

func TestLayoutStylesStayLocalAndOverridesAreExplicit(t *testing.T) {
	t.Parallel()

	outer, err := parseLayout("outer.kui", []byte(`<template><panel class="content"><label>Outer</label><slot/></panel></template>
<style>
.content
  gap: 8px
label
  color: #112233
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	inner, err := parseLayout("inner.kui", []byte(`<template><panel class="content"><label>Inner</label><slot/></panel></template>
<style>
.content
  gap: 24px
label
  color: #445566
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	markup := `<template><Outer><label class="caption">Caller</label><Inner><label>Fill</label></Inner><Inner><label>Second</label></Inner></Outer></template>`

	component, err := compileWithLayouts("menu.kui", []byte(markup+`
<style>
.caption
  color: #ffffff
</style>`), DefaultTheme(), map[string]Layout{"Outer": outer, "Inner": inner})
	if err != nil {
		t.Fatal(err)
	}

	elements := component.Template.Elements
	if component.Template.Panel.Gap != 8 || elements[2].Style.Gap != 24 || elements[5].Style.Gap != 24 ||
		elements[0].Style.Color != 0x112233ff ||
		elements[1].Style.Color != 0xffffffff ||
		elements[3].Style.Color != 0x445566ff ||
		elements[4].Style.Set&ui.StyleColor != 0 {
		t.Fatalf("style escaped its owner: root=%+v elements=%+v", component.Template.Panel, elements)
	}

	component, err = compileWithLayouts("menu.kui", []byte(markup+`
<style>
.caption
  color: #ffffff
Outer.content
  gap: 12px
Inner.content
  gap: 20px
@media (max-width: 480)
  Inner.content
    gap: 16px
</style>`), DefaultTheme(), map[string]Layout{"Outer": outer, "Inner": inner})
	if err != nil || len(component.StyleWarnings()) != 0 || component.Template.Panel.Gap != 12 ||
		component.Template.Elements[2].Style.Gap != 20 ||
		component.Template.Elements[2].ResponsiveStyle.Gap != 16 ||
		component.Template.Elements[5].ResponsiveStyle.Gap != 16 ||
		component.Template.ResponsivePanel.Set != 0 {
		t.Fatalf("explicit/responsive scope override: %v %+v %v", err, component.Template.Panel.Gap, component.StyleWarnings())
	}

	if _, err := ui.EncodeComposition(component.Template); err != nil {
		t.Fatal(err)
	}
}

func TestStyleDiagnosticsUseOriginalLocations(t *testing.T) {
	t.Parallel()

	for _, entry := range []struct {
		text         string
		line, column int
	}{
		{"<!-- Header -->\n<template><panel class=\"screen\"><label>Ready</label></panel></template>\n<style>\n.screen\n  width: 9000px\n</style>", 5, 3},
		{"<template><panel class=\"screen\"><label>Ready</label></panel></template>\n<style>\n.screen { gap: 8; width: 9000px; }\n</style>", 3, 19},
		{"kartui Menu() { <panel><label>Ready</label></panel> }\nstyle { panel { width: 9000px; } }", 2, 17},
		{"<template><panel><label>Ready</label></panel></template>\n<style>\n$space: $missing\npanel\n  gap: 8\n</style>", 3, 1},
	} {
		component, err := Compile("ui/menu.kui", []byte(entry.text))
		if err != nil {
			t.Fatal(err)
		}

		diagnostics := component.Diagnostics()
		if len(diagnostics) != 1 || diagnostics[0].Source != "ui/menu.kui" || diagnostics[0].Line != entry.line ||
			diagnostics[0].Column != entry.column {
			t.Fatalf("incorrect location: %+v for %s", diagnostics, entry.text)
		}

		before := component.StyleWarnings()[0]
		diagnostics[0].Message = "changed"

		if component.StyleWarnings()[0] != before {
			t.Fatal("diagnostics alias compiler storage")
		}
	}
}

func TestNestedStateAndLayoutDiagnosticsKeepOrigins(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout(
		"ui/layouts/frame.kui",
		[]byte(
			"<!-- Layout -->\n<template><panel class=\"screen\"><slot/></panel></template>\n<style lang='sass'>\n.screen\n  width: 9000px\n</style>",
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	component, err := compileWithLayouts(
		"menu.kui",
		[]byte(`<template><Frame><button onClick={func(){}} class="action">Play</button></Frame></template>
<style>
.action
  &:hover
    height: 100%
</style>`),
		DefaultTheme(),
		map[string]Layout{"Frame": layout},
	)
	if err != nil {
		t.Fatal(err)
	}

	diagnostics := component.Diagnostics()
	if len(diagnostics) != 2 || diagnostics[0].Source != "ui/layouts/frame.kui" || diagnostics[0].Line != 5 || diagnostics[0].Column != 3 ||
		diagnostics[1].Source != "menu.kui" ||
		diagnostics[1].Line != 5 ||
		diagnostics[1].Column != 5 {
		t.Fatalf("lost source provenance: %+v", diagnostics)
	}
}

func TestCompilerErrorsPointToAuthoredSFC(t *testing.T) {
	t.Parallel()

	for _, entry := range []struct {
		text         string
		line, column int
	}{
		{"<!-- Header -->\n<template><panel><label>Ready</label></panel></template>\n<style>\npanel\n  width: 80%\n gap: 8\n</style>", 6, 2},
		{"<!-- Header -->\n<script lang='go' setup>\nfunc setup() {\n name :=\n}\n</script>\n<template><panel><label>Ready</label></panel></template>", 5, 1},
		{"<!-- Header -->\n<template>\n<panel>\n  <label class=\"one two\">Ready</label>\n</panel>\n</template>", 4, 3},
	} {
		_, err := Compile("ui/broken.kui", []byte(entry.text))

		var failure *SourceError
		if !errors.As(err, &failure) || failure.Line != entry.line || failure.Column != entry.column ||
			!strings.Contains(err.Error(), "ui/broken.kui:") {
			t.Fatalf("wrong error location: %v expected %d:%d", err, entry.line, entry.column)
		}
	}
}

func TestCompilerMetadataCannotBeAuthored(t *testing.T) {
	t.Parallel()

	for _, source := range []string{`<template><panel _kartyOrigin="0"><label>Text</label></panel></template>`, `kartui Menu() { <panel><label _kartyOrigin="0">Text</label></panel> }`} {
		if _, err := Compile("bad.kui", []byte(source)); !errors.Is(err, ui.ErrTemplate) {
			t.Fatal("accepted forged metadata", err)
		}
	}
}

func TestMarkupExpressionErrorsAndWarningsRemainBounded(t *testing.T) {
	t.Parallel()

	_, err := Compile("menu.kui", []byte("<!-- Header -->\n<template>\n<panel>\n  <label>{ + }</label>\n</panel>\n</template>"))

	var failure *SourceError
	if !errors.As(err, &failure) || failure.Line != 4 || failure.Column != 10 {
		t.Fatalf("expression location: %v", err)
	}

	source := strings.Repeat("\u754c", maxStyleWarningBytes)

	component, err := Compile(
		source+"/menu.kui",
		[]byte("<template><panel><label>Ready</label></panel></template>\n<style>\npanel\n  width: 9000px\n</style>"),
	)
	if err != nil {
		t.Fatal(err)
	}

	warning := component.StyleWarnings()[0]
	if len(warning) > maxStyleWarningBytes+len("...") || !utf8.ValidString(warning) {
		t.Fatal("unbounded or invalid UTF-8 warning")
	}
}

func TestMarkupOriginsSurviveExpressionLoweringAndSlotProjection(t *testing.T) {
	t.Parallel()

	for _, entry := range []struct {
		text string
		line int
	}{
		{"<template>\n<panel>\n<button onClick={func() {\n println(\"ready\")\n}}>Ready</button>\n<label></button>\n</panel>\n</template>", 6},
		{"<template>\n<panel>\nif + {\n<label>Ready</label>\n}\n</panel>\n</template>", 3},
		{"<template>\n<panel>\nif ready {\n<label>Ready</label>\n} else {\n<label></button>\n}\n</panel>\n</template>", 6},
	} {
		_, err := Compile("broken.kui", []byte(entry.text))

		failure, found := errors.AsType[*SourceError](err)
		if !found || failure.Line != entry.line {
			t.Fatalf("source location after lowering: %v expected line %d", err, entry.line)
		}
	}

	layout, err := parseLayout("frame.kui", []byte("<template><panel><slot/></panel></template>"))
	if err != nil {
		t.Fatal(err)
	}

	_, err = compileWithLayouts(
		"menu.kui",
		[]byte("<!-- Header -->\n<template>\n<Frame>\n  <label class=\"two classes\">Text</label>\n</Frame>\n</template>"),
		DefaultTheme(),
		map[string]Layout{"Frame": layout},
	)

	failure, found := errors.AsType[*SourceError](err)
	if !found || failure.Source != "menu.kui" || failure.Line != 4 || failure.Column != 3 {
		t.Fatal("lost projected caller location", err)
	}
}
