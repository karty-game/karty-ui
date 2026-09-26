package ui

import (
	"reflect"
	"strings"
	"testing"
)

func TestTemplate(t *testing.T) {
	t.Parallel()

	valid := Template{
		Version:  1,
		Modal:    true,
		Elements: []Element{{ID: 1, Name: "title", Kind: "label", Text: "Inventory"}, {ID: 2, Name: "items", Kind: "list", Action: 1}},
	}

	data, err := Encode(valid)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := Decode(data)
	if err != nil || !reflect.DeepEqual(valid, decoded) {
		t.Fatalf("roundtrip: %+v %v", decoded, err)
	}

	for index := range data {
		if _, err := Decode(data[:index]); err == nil {
			t.Fatalf("accepted truncation %d", index)
		}
	}

	for _, data := range []string{
		`{"version":2,"elements":[]}`,
		`{"version":1,"extra":true,"elements":[]}`,
		`{"version":1,"elements":[{"id":1,"name":"a","kind":"button","action":1}]}`,
		`{"version":1,"elements":[{"id":1,"name":"a","kind":"label"},{"id":1,"name":"b","kind":"label"}]}`,
		`{"version":1,"elements":[{"id":1,"name":"a","kind":"label"}]} {}`,
		strings.Repeat(" ", MaxAssetBytes+1),
	} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatal("accepted invalid template")
		}
	}
}

func TestLayoutTemplate(t *testing.T) {
	t.Parallel()

	template := Template{
		Version: SchemaLayout,
		Modal:   true,
		Panel: Style{
			Set: StyleTransitionDuration, TransitionDuration: 180,
		},
		Elements: []Element{
			{ID: 1, Name: "window", Kind: "panel"},
			{ID: 2, Parent: 1, Name: "content", Kind: "panel", Style: Style{
				Set: StyleFlexDirection | StyleAlignItems | StyleMaxWidth | StyleJustifyContent |
					StyleFlexGrow | StyleMargin | StyleOverflow,
				FlexDirection: FlexDirectionRow, AlignItems: AlignItemsStretch, MaxWidth: 640,
				JustifyContent: JustifyContentSpaceBetween, FlexGrow: 2, Margin: 6, Overflow: OverflowScroll,
			}},
			{ID: 3, Parent: 2, Name: "play", Kind: "button", Text: "Play", Action: 1,
				Style: Style{Set: StyleBackgroundDisabled | StyleColorHover}},
		},
	}

	data, err := EncodeComposition(template)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeComposition(data)
	if err != nil || !reflect.DeepEqual(decoded, template) {
		t.Fatalf("layout roundtrip: %+v, %v", decoded, err)
	}

	invalid := template
	invalid.Elements = append([]Element(nil), template.Elements...)

	invalid.Elements[1].Parent = 99
	if err := invalid.ValidateComposition(); err == nil {
		t.Fatal("accepted missing parent")
	}

	old := template

	old.Version = SchemaImageStyle
	if err := old.ValidateComposition(); err == nil {
		t.Fatal("accepted layout data in an old schema")
	}

	invalid = template
	invalid.Elements = append([]Element(nil), template.Elements...)

	invalid.Elements[1].Style.FlexDirection = 99
	if err := invalid.ValidateComposition(); err == nil {
		t.Fatal("accepted invalid flex direction")
	}

	invalid.Elements[1].Style.FlexDirection = FlexDirectionRow
	invalid.Elements[1].Style.FlexGrow = 17

	if err := invalid.ValidateComposition(); err == nil {
		t.Fatal("accepted excessive flex growth")
	}

	invalid.Elements[1].Style.FlexGrow = 2
	invalid.Elements[1].Style.Overflow = 99

	if err := invalid.ValidateComposition(); err == nil {
		t.Fatal("accepted invalid overflow")
	}

	invalid = template
	invalid.Elements = append([]Element(nil), template.Elements...)
	invalid.Elements[1].Style.Set |= StyleTransitionDuration
	invalid.Elements[1].Style.TransitionDuration = 100

	if err := invalid.ValidateComposition(); err == nil {
		t.Fatal("accepted a nested view transition")
	}
}

func TestResponsiveTemplate(t *testing.T) {
	t.Parallel()

	template := Template{
		Version: SchemaResponsive, Modal: true,
		Panel:              Style{Set: StylePadding, Padding: 24},
		ResponsiveMaxWidth: 480,
		ResponsivePanel:    Style{Set: StylePadding, Padding: 8},
		Elements: []Element{{
			ID: 1, Name: "title", Kind: "label", Text: "Menu",
			Style:           Style{Set: StyleFontSize, FontSize: 30},
			ResponsiveStyle: Style{Set: StyleFontSize | StyleMaxWidth, FontSize: 24, MaxWidth: 300},
		}},
	}

	data, err := EncodeComposition(template)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeComposition(data)
	if err != nil || !reflect.DeepEqual(decoded, template) {
		t.Fatalf("responsive roundtrip: %+v, %v", decoded, err)
	}

	for _, mutate := range []func(*Template){
		func(value *Template) { value.Version = SchemaLayout },
		func(value *Template) { value.ResponsiveMaxWidth = MinResponsiveWidth - 1 },
		func(value *Template) { value.ResponsiveMaxWidth = MaxResponsiveWidth + 1 },
		func(value *Template) { value.Elements[0].ResponsiveStyle.Set = StylePadding },
	} {
		invalid := template
		invalid.Elements = append([]Element(nil), template.Elements...)
		mutate(&invalid)

		if err := invalid.ValidateComposition(); err == nil {
			t.Fatal("accepted invalid responsive template")
		}
	}
}

func TestControlTransitionTemplate(t *testing.T) {
	t.Parallel()

	template := Template{
		Version: SchemaControlTransition,
		Modal:   true,
		Elements: []Element{
			{
				ID: 1, Name: "drawer", Kind: "panel",
				Style: Style{
					Set:                StyleTransitionDuration | StyleTransitionEnter | StyleTransitionExit,
					TransitionDuration: 180, TransitionEnter: TransitionLeft, TransitionExit: TransitionRight,
				},
			},
			{
				ID: 2, Parent: 1, Name: "play", Kind: "button", Text: "Play", Action: 1,
				Style: Style{Set: StyleTransitionDuration, TransitionDuration: 140},
			},
		},
	}

	data, err := EncodeComposition(template)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeComposition(data)
	if err != nil || !reflect.DeepEqual(decoded, template) {
		t.Fatalf("control transition roundtrip: %+v, %v", decoded, err)
	}

	template.Version = SchemaResponsive
	if err := template.ValidateComposition(); err == nil {
		t.Fatal("responsive schema accepted a control transition")
	}
}

func TestTextAlignmentTemplate(t *testing.T) {
	t.Parallel()

	template := Template{
		Version: SchemaVisualHierarchy,
		Modal:   true,
		Elements: []Element{{
			ID: 1, Name: "play", Kind: "button", Text: "Play", Action: 1,
			Style: Style{Set: StyleTextAlign, TextAlign: TextAlignStart},
		}},
	}

	if _, err := EncodeComposition(template); err != nil {
		t.Fatal(err)
	}

	template.Version = SchemaControlTransition
	if err := template.ValidateComposition(); err == nil {
		t.Fatal("control-transition schema accepted text alignment")
	}

	template.Version = SchemaVisualHierarchy

	template.Elements[0].Style.TextAlign = 99
	if err := template.ValidateComposition(); err == nil {
		t.Fatal("accepted invalid text alignment")
	}

	template.Elements[0].Style = Style{Set: StyleFontFamily, FontFamily: FontFamilyDisplay}
	if _, err := EncodeComposition(template); err != nil {
		t.Fatal(err)
	}
}

func TestForegroundImageRequiresCurrentSchema(t *testing.T) {
	t.Parallel()

	template := Template{
		Version: SchemaVisualHierarchy,
		Elements: []Element{{
			ID: 1, Name: "emblem", Kind: "image",
			Style: Style{
				Set:          StyleContentImage,
				ContentImage: Image{Scope: ImageScopeGame, Name: "ui.emblem"},
			},
		}},
	}

	if _, err := EncodeComposition(template); err != nil {
		t.Fatal(err)
	}

	template.Version = SchemaControlTransition
	if err := template.ValidateComposition(); err == nil {
		t.Fatal("older schema accepted a foreground image")
	}
}

func TestInteractionPolishRequiresCurrentSchema(t *testing.T) {
	t.Parallel()

	template := Template{
		Version: SchemaInteractionPolish,
		Modal:   true,
		Elements: []Element{{
			ID: 1, Name: "play", Kind: "button", Text: "Play", Action: 1,
			Style: Style{
				Set: StyleTransitionDuration, TransitionDuration: 180,
				Set2:            Style2BackgroundFocus | Style2ColorFocus | Style2TransitionDelay | Style2TransitionEasing,
				BackgroundFocus: 0x70d6ffff, ColorFocus: 0x06101bff,
				TransitionDelay: 40, TransitionEasing: TransitionEasingInOut,
			},
		}},
	}

	if _, err := EncodeComposition(template); err != nil {
		t.Fatal(err)
	}

	template.Version = SchemaVisualHierarchy
	if err := template.ValidateComposition(); err == nil {
		t.Fatal("visual-hierarchy schema accepted interaction polish")
	}
}

func TestOverlayPositionRequiresInteractionSchema(t *testing.T) {
	t.Parallel()

	template := Template{
		Version: SchemaInteractionPolish,
		Elements: []Element{{
			ID: 1, Name: "badge", Kind: "panel",
			Style: Style{
				Set2:     Style2Position | Style2Top | Style2Right,
				Position: PositionOverlay, Top: 12, Right: 16,
			},
		}},
	}

	if _, err := EncodeComposition(template); err != nil {
		t.Fatal(err)
	}

	template.Elements[0].Style.Set2 &^= Style2Position
	if err := template.ValidateComposition(); err == nil {
		t.Fatal("accepted overlay inset without absolute position")
	}
}

func TestImagePresentationRequiresInteractionSchema(t *testing.T) {
	t.Parallel()

	icon := Image{Scope: ImageScopeGame, Name: "ui.play"}
	template := Template{
		Version: SchemaInteractionPolish,
		Modal:   true,
		Elements: []Element{
			{ID: 1, Name: "art", Kind: "image", Style: Style{
				Set: StyleContentImage, ContentImage: icon,
				Set2: Style2ImageFit | Style2Tint, ImageFit: ImageFitContain, Tint: 0xffffffff,
			}},
			{ID: 2, Name: "play", Kind: "button", Text: "Play", Action: 2, Style: Style{
				Set2: Style2Icon | Style2IconSize | Style2IconPosition | Style2IconGap,
				Icon: icon, IconSize: 20, IconPosition: IconPositionStart, IconGap: 8,
			}},
		},
	}

	if _, err := EncodeComposition(template); err != nil {
		t.Fatal(err)
	}

	template.Version = SchemaVisualHierarchy
	if err := template.ValidateComposition(); err == nil {
		t.Fatal("visual-hierarchy schema accepted image presentation polish")
	}
}
