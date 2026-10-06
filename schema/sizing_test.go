package ui

import "testing"

func TestDimensionSchemaRejectsMalformedOrUnversionedLengths(t *testing.T) {
	t.Parallel()

	valid := Template{
		Elements: []Element{{ID: 1, Name: "label", Kind: "label", Text: "Size"}},
		Version:  SchemaSizing,
		Panel:    Style{Set2: Style2Width, Width: Length{Unit: LengthPercent, Value: 8000}},
	}
	if err := valid.ValidateComposition(); err != nil {
		t.Fatal(err)
	}

	for _, invalid := range []Style{
		{Set2: Style2Width, Width: Length{Unit: LengthPercent, Value: 10001}},
		{Set2: Style2Height, Height: Length{Unit: LengthPixels, Value: 2049}},
		{Set2: Style2Width, Width: Length{Unit: LengthAuto, Value: 1}},
		{Set2: Style2Width, Width: Length{Unit: 3}},
		{Width: Length{Unit: LengthPixels, Value: 1}},
		{Set2: Style2Width, Set: StyleMinWidth | StyleMaxWidth, MinWidth: 30, MaxWidth: 20},
	} {
		candidate := valid

		candidate.Panel = invalid
		if err := candidate.ValidateComposition(); err == nil {
			t.Fatalf("accepted %+v", invalid)
		}
	}

	valid.Version = SchemaWidgets
	if err := valid.ValidateComposition(); err == nil {
		t.Fatal("old schema accepted dimensions")
	}
}
