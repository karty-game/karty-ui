package ui

import (
	"reflect"
	"testing"
)

func TestInteractiveNonmodalHUDRequiresSchemaNine(t *testing.T) {
	t.Parallel()

	for _, kind := range []string{"button", "list"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()

			template := Template{Version: SchemaInteractionPolish, Elements: []Element{{ID: 1, Name: "move", Kind: kind, Action: 1}}}

			encoded, err := EncodeComposition(template)
			if err != nil {
				t.Fatal(err)
			}

			decoded, err := DecodeComposition(encoded)
			if err != nil || !reflect.DeepEqual(decoded, template) {
				t.Fatalf("nonmodal action roundtrip: %+v %v", decoded, err)
			}

			for version := uint32(1); version < SchemaInteractionPolish; version++ {
				template.Version = version
				if err := template.ValidateComposition(); err == nil {
					t.Fatalf("schema %d accepted an interactive nonmodal HUD", version)
				}
			}

			template.Version = SchemaInteractionPolish

			template.Back = true
			if err := template.ValidateComposition(); err == nil {
				t.Fatal("nonmodal HUD captured semantic Back")
			}

			template.Back = false

			template.Elements[0].Action = 0
			if err := template.ValidateComposition(); err == nil {
				t.Fatal("nonmodal control accepted an absent action")
			}
		})
	}
}
