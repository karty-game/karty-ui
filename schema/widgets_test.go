package ui

import "testing"

func TestWidgetSchemaVersionAndFieldOwnership(t *testing.T) {
	t.Parallel()

	valid := Template{
		Version:  SchemaWidgets,
		Elements: []Element{{ID: 1, Name: "checkbox", Kind: "checkbox", Text: "Audio", Value: "false", Action: 1}},
	}
	if err := valid.ValidateComposition(); err != nil {
		t.Fatal(err)
	}

	for _, change := range []func(*Template){
		func(template *Template) { template.Version = SchemaInteractionPolish },
		func(template *Template) { template.Elements[0].Value = "False" },
		func(template *Template) { template.Elements[0].Min = 1 },
		func(template *Template) { template.Elements[0].Placeholder = "wrong widget" },
		func(template *Template) { template.Elements[0].Style = Style{Set2: Style2ColorFocus} },
	} {
		candidate := valid
		candidate.Elements = append([]Element(nil), valid.Elements...)
		change(&candidate)

		if err := candidate.ValidateComposition(); err == nil {
			t.Fatal("accepted invalid schema", candidate)
		}
	}

	label := Template{Version: SchemaInteractionPolish, Elements: []Element{{ID: 1, Name: "label", Kind: "label", Tooltip: "Help"}}}
	if err := label.ValidateComposition(); err == nil {
		t.Fatal("old schema accepted tooltip")
	}

	label.Version = SchemaWidgets
	if err := label.ValidateComposition(); err != nil {
		t.Fatal(err)
	}
}
