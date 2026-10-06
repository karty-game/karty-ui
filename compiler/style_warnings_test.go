package uicompiler

import (
	"errors"
	"strings"
	"testing"

	ui "github.com/karty-game/karty-ui/schema"
)

func TestStyleWarningsKeepValidDeclarationsAndLastValidValue(t *testing.T) {
	t.Parallel()

	for _, source := range []string{
		`<template>
<panel class="screen"><label class="copy">Text</label></panel>
</template>

<style>
.screen
  width: 80%
  padding: 8
  gap: 12
.screen
  width: 9000px
  padding: 257
  gap: -1
  unknown: 1
.copy
  color: #ffffffff
  padding: 8
  font-size: 129
  width: 120%
</style>`,
		`<template><panel class="screen"><label class="copy">Text</label></panel></template>
<style>
.screen
  width: 80%
  padding: 8px
  gap: 12px
.screen
  width: 9000px
  padding: 257px
  gap: -1
  unknown: 1
.copy
  color: #ffffffff
  padding: 8px
  font-size: 129px
  width: 120%
</style>`,
	} {
		component, err := Compile("ui/menu.kui", []byte(source))
		if err != nil {
			t.Fatal(err)
		}

		panel, label := component.Template.Panel, component.Template.Elements[0].Style
		if panel.Width.Value != 8000 || panel.Padding != 8 || panel.Gap != 12 {
			t.Fatalf("invalid rule replaced valid values: %+v", panel)
		}

		if label.Set != ui.StyleColor || label.Color != 0xffffffff || label.Set2 != 0 {
			t.Fatalf("invalid target styles leaked: %+v", label)
		}

		warnings := component.StyleWarnings()
		if len(warnings) != 7 {
			t.Fatalf("warnings=%v", warnings)
		}

		for _, warning := range warnings {
			if !strings.Contains(warning, "ui/menu.kui:") || !strings.Contains(warning, "ignored:") {
				t.Fatal(warning)
			}
		}

		encoded, err := ui.EncodeComposition(component.Template)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := ui.DecodeComposition(encoded); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInvalidResponsiveStylesDoNotOverwriteTheBase(t *testing.T) {
	t.Parallel()

	component, err := Compile("menu.kui", []byte(`<template><panel class="screen"><label>Text</label></panel></template>
<style>
.screen
  width: 80%
  padding: 8px
@media (max-width: 480)
  .screen
    width: 99999px
    padding: 300px
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.ResponsiveMaxWidth != 0 || component.Template.Panel.Width.Value != 8000 || len(component.StyleWarnings()) != 2 {
		t.Fatalf("invalid responsive declarations did not fall back: %+v %v", component.Template, component.StyleWarnings())
	}
}

func TestStyleWarningsResolveDependenciesAndConflictsAcrossRules(t *testing.T) {
	t.Parallel()

	component, err := Compile(
		"menu.kui",
		[]byte(
			`<template>
<panel><panel class="scroll"><label class="copy">Text</label></panel><button class="action" onClick={Click}>Action</button></panel>
</template>

<script setup lang="go">
func setup(Click func()) {}
</script>

<style>
.scroll
  gap: 8
  overflow: scroll
  transition-duration: 100
  transition-enter: slide-from-left
  top: 8
.copy
  min-width: 100
  max-width: 50
  color: #ffffff
.action
  icon-size: 20
  transition-delay: 100
  color: #ffffff
</style>`,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	panel, label, button := component.Template.Elements[0].Style, component.Template.Elements[1].Style, component.Template.Elements[2].Style
	if panel.Set != ui.StyleGap|ui.StyleOverflow || panel.Set2 != 0 {
		t.Fatalf("invalid panel dependencies: %+v", panel)
	}

	if label.MinWidth != 100 || label.Set&ui.StyleMaxWidth != 0 || label.Set&ui.StyleColor == 0 {
		t.Fatalf("conflicting max-width was retained: %+v", label)
	}

	if button.Set2 != 0 || button.IconSize != 0 || button.TransitionDelay != 0 || button.Color != 0xffffffff {
		t.Fatalf("orphan control fields: %+v", button)
	}

	if len(component.StyleWarnings()) != 6 {
		t.Fatalf("dependency warnings=%v", component.StyleWarnings())
	}
}

func TestSharedStyleClassIsFilteredPerWidget(t *testing.T) {
	t.Parallel()

	component, err := Compile(
		"menu.kui",
		[]byte(
			`<template>
<panel><label class="shared">Text</label><button class="shared" onClick={Click}>Action</button></panel>
</template>

<script setup lang="go">
func setup(Click func()) {}
</script>

<style>
.shared
  color: #ffffff
  background: #000000
  width: 50%
</style>`,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	label, button := component.Template.Elements[0].Style, component.Template.Elements[1].Style
	if label.Set&ui.StyleBackground != 0 || button.Set&ui.StyleBackground == 0 || label.Width.Value != 5000 || button.Width.Value != 5000 {
		t.Fatalf("shared selector lost valid widget-specific styling")
	}

	if len(component.StyleWarnings()) != 1 {
		t.Fatal(component.StyleWarnings())
	}
}

func TestLayoutStyleWarningsKeepOwningSourceAndPrecedence(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("ui/layouts/frame.kui", []byte(`<template><panel class="screen"><slot/></panel></template>
<style>
.screen
  width: 9999px
  gap: 8px
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	component, err := compileWithLayouts("ui/menu.kui", []byte(`<template><Frame><label>Text</label></Frame></template>
<style>
Frame.screen
  gap: 12px
</style>`), DefaultTheme(), map[string]Layout{"Frame": layout})
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Panel.Gap != 12 || len(component.StyleWarnings()) != 1 ||
		!strings.Contains(component.StyleWarnings()[0], "ui/layouts/frame.kui:") {
		t.Fatalf("layout diagnostics/precedence: %v %+v", component.StyleWarnings(), component.Template.Panel)
	}
}

func TestSassSemanticStyleWarningsContinueWithValidRules(t *testing.T) {
	t.Parallel()

	component, err := Compile("menu.kui", []byte(`<template><panel class="screen"><label>Text</label></panel></template>
<style>
.screen
  width: $missing
  gap: 8px
  direction: row
  flex-direction: column
  button
    padding: 1px
@media (max-width: 5000)
  .screen
    gap: 16px
</style>`))
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Panel.Gap != 8 || component.Template.Panel.FlexDirection != ui.FlexDirectionRow ||
		component.Template.ResponsiveMaxWidth != 0 ||
		len(component.StyleWarnings()) != 4 {
		t.Fatalf("Sass fallback: %+v %v", component.Template.Panel, component.StyleWarnings())
	}
}

func TestStyleWarningsAreBoundedAndReturnedAsCopies(t *testing.T) {
	t.Parallel()

	var body strings.Builder
	for index := range maxStyleWarnings + 10 {
		body.WriteString("bad-")
		body.WriteString(strings.Repeat("x", index+1))
		body.WriteString(": 1\n  ")
	}

	component, err := Compile(
		"menu.kui",
		[]byte(`<template>
<panel><label>Text</label></panel>
</template>

<style>
panel
  `+body.String()+`
</style>`),
	)
	if err != nil {
		t.Fatal(err)
	}

	warnings := component.StyleWarnings()
	if len(warnings) != maxStyleWarnings+1 {
		t.Fatalf("unbounded warnings=%d", len(warnings))
	}

	warnings[0] = "modified"

	if component.StyleWarnings()[0] == "modified" {
		t.Fatal("diagnostic caller modified compiler storage")
	}
}

func TestMalformedAssetsAndStaticBindingsRemainErrors(t *testing.T) {
	t.Parallel()

	template := ui.Template{
		Version:  ui.SchemaSizing,
		Panel:    ui.Style{Set2: ui.Style2Width, Width: ui.Length{Unit: ui.LengthPixels, Value: 9000}},
		Elements: []ui.Element{{ID: 1, Name: "label", Kind: "label", Text: "Text"}},
	}
	if _, err := ui.EncodeComposition(template); !errors.Is(err, ui.ErrTemplate) {
		t.Fatal("invalid asset bypassed strict encoding")
	}

	if _, err := (&Component{Source: "malformed.kui", Template: template}).StaticTemplate(); !errors.Is(err, ui.ErrTemplate) {
		t.Fatal("invalid component bypassed strict static validation")
	}

	if _, err := DecodeSource(
		"level.kui",
		[]byte(`<template>
<panel><label>{Title}</label></panel>
</template>

<script setup lang="go">
func setup(Title string) {}
</script>`),
	); !errors.Is(
		err,
		ui.ErrTemplate,
	) {
		t.Fatal("dynamic level bindings were accepted")
	}
}

func TestInvalidSassVariableDeclarationsWarnWithoutLosingValidValues(t *testing.T) {
	t.Parallel()

	component, err := Compile("menu.kui", []byte(`<template><panel class="screen"><label>Text</label></panel></template>
<style>
$bad: $later
$gap: 8px
$gap: 16px
.screen
  gap: $gap
  width: $bad
  color: $missing
</style>`))
	if err != nil || component.Template.Panel.Gap != 8 || len(component.StyleWarnings()) != 4 {
		t.Fatalf("variable recovery: %v %+v %v", err, component.Template.Panel, component.StyleWarnings())
	}
}

func TestIndependentStylePropertyPrerequisitesAreOrderIndependent(t *testing.T) {
	t.Parallel()

	theme := DefaultTheme()

	theme.Images["icon"] = ui.Image{Scope: ui.ImageScopeGame, Name: "icon"}
	for _, declarations := range []string{
		"top: 8\n  position: absolute\n  transition-delay: 1\n  transition-enter: slide-from-left\n  transition-duration: 100",
		"transition-duration: 100\n  transition-enter: slide-from-left\n  transition-delay: 1\n  position: absolute\n  top: 8",
	} {
		component, err := CompileWithTheme(
			"menu.kui",
			[]byte(
				`<template>
<panel><panel class="nested"><label>Text</label></panel></panel>
</template>

<style>
.nested
  `+declarations+`
</style>`,
			),
			theme,
		)
		if err != nil || len(component.StyleWarnings()) != 0 {
			t.Fatalf("valid prerequisites were ignored: %v %v", err, component.StyleWarnings())
		}
	}
}

func TestInvalidDeclarationDoesNotReserveProperty(t *testing.T) {
	t.Parallel()

	component, err := Compile("fallback.kui", []byte(`
<template><panel><label>Ready</label></panel></template>
<style lang="sass">
panel
  width: 9999px
  width: 80%
  width: 40%
</style>
`))
	if err != nil || component.Template.Panel.Width.Value != 8000 || len(component.StyleWarnings()) != 2 {
		t.Fatalf("ignored declaration reserved its property: %v %+v %v", err, component.Template.Panel, component.StyleWarnings())
	}
}

func TestStyleRangeWarningsAcrossNumericProperties(t *testing.T) {
	t.Parallel()

	for property, value := range map[string]string{
		"width": "2049px", "height": "100.01%", "padding": "257", "gap": "257", "margin": "257",
		"font-size": "129", "min-height": "257", "min-width": "2049", "max-width": "2049", "max-height": "2049",
		"grow": "17", "left": "2049", "right": "2049", "top": "2049", "bottom": "2049",
		"icon-size": "257", "icon-gap": "257", "transition-duration": "2001", "transition-delay": "2001",
	} {
		t.Run(property, func(t *testing.T) {
			t.Parallel()

			component, err := Compile("range.kui", []byte(`<template><panel><label>Ready</label></panel></template>
<style lang="sass">
panel
  `+property+`: `+value+`
  padding: 8
</style>`))
			if err != nil || component.Template.Panel.Padding != 8 || len(component.StyleWarnings()) != 1 {
				t.Fatalf(
					"invalid range caused failure or lost valid padding: %v %+v %v",
					err,
					component.Template.Panel,
					component.StyleWarnings(),
				)
			}

			if _, err := ui.EncodeComposition(component.Template); err != nil {
				t.Fatal(err)
			}
		})
	}
}
