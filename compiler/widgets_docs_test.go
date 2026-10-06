package uicompiler

import (
	"os"
	"strings"
	"testing"
)

func TestWidgetReferenceCompiles(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("../docs/widgets.md")
	if err != nil {
		t.Fatal(err)
	}

	_, example, found := strings.Cut(string(contents), "```kui\n")
	if !found {
		t.Fatal("missing widget example")
	}

	example, _, found = strings.Cut(example, "```")
	if !found {
		t.Fatal("unclosed widget example")
	}

	if _, err := Compile("settings.kui", []byte(example)); err != nil {
		t.Fatal(err)
	}
}
