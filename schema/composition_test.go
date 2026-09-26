package ui

import "testing"

func TestCompositionSchemaIsolation(t *testing.T) {
	t.Parallel()

	template := Template{Version: 2, Elements: []Element{{ID: 1, Name: "children", Kind: "slot"}}}
	if template.ValidateComposition() != nil || template.Validate() == nil {
		t.Fatal("schema 2 must validate only through explicit composition entry point")
	}

	for _, invalid := range []Element{
		{ID: 1, Name: "children", Kind: "slot", Text: "hidden"},
		{ID: 1, Name: "children", Kind: "slot", Action: 1},
	} {
		template.Elements[0] = invalid
		if template.ValidateComposition() == nil {
			t.Fatal("slot accepted text/action payload")
		}
	}
}

func TestStyleSchemaIsolation(t *testing.T) {
	t.Parallel()

	template := Template{Version: 3, Modal: true, Panel: Style{Set: StylePadding, Padding: 24}, Elements: []Element{{
		ID: 1, Name: "play", Kind: "button", Text: "Play", Action: 1,
		Style: Style{Set: StyleBackground | StyleColor, Background: 0x112233ff, Color: 0xffffffff},
	}}}

	encoded, err := EncodeComposition(template)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeComposition(encoded)
	if err != nil || decoded.Panel.Padding != 24 || decoded.Elements[0].Style.Background != 0x112233ff {
		t.Fatalf("style round trip: %+v %v", decoded, err)
	}

	if template.Validate() == nil {
		t.Fatal("legacy decoder accepted schema 3")
	}

	template.Elements[0].Style.Set = StylePadding
	if template.ValidateComposition() == nil {
		t.Fatal("button accepted panel-only padding")
	}
}

func TestImageStyleSchemaIsolation(t *testing.T) {
	t.Parallel()

	image := Image{Scope: ImageScopeGame, Name: "ui.panel", Top: 8, Right: 8, Bottom: 8, Left: 8}
	template := Template{
		Version: 4, Modal: true,
		Panel:    Style{Set: StyleBackgroundImage, BackgroundImage: image},
		Elements: []Element{{ID: 1, Name: "title", Kind: "label", Text: "Menu"}},
	}

	encoded, err := EncodeComposition(template)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := DecodeComposition(encoded)
	if err != nil || decoded.Panel.BackgroundImage != image {
		t.Fatalf("image style round trip: %+v %v", decoded, err)
	}

	template.Version = 3
	if template.ValidateComposition() == nil {
		t.Fatal("schema 3 accepted image-backed style")
	}

	template.Version = 4

	template.Panel.BackgroundImage = Image{Scope: ImageScopeLevel}
	if template.ValidateComposition() == nil {
		t.Fatal("invalid level image reference accepted")
	}
}
