package uicompiler

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestDefaultAuthoringReferenceAndSettingsSnippetCompile(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"../README.md", "../docs/0.0.1/kartui.md", "../docs/widgets.md"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}

		marker := "```kui\n"
		if path == "../README.md" {
			marker = "```html\n"
		}

		_, example, found := strings.Cut(string(data), marker)
		if !found {
			t.Fatal("missing SFC example", path)
		}

		example, _, found = strings.Cut(example, "```")
		if !found {
			t.Fatal("unclosed SFC example", path)
		}

		component, err := Compile("example.kui", []byte(example))
		if err != nil || len(component.StyleWarnings()) != 0 {
			t.Fatalf("%s: %v %v", path, err, component.StyleWarnings())
		}
	}

	data, err := os.ReadFile("../editors/vscode/snippets/kartui.json")
	if err != nil {
		t.Fatal(err)
	}

	var snippets map[string]struct {
		Body []string `json:"body"`
	}
	if err := json.Unmarshal(data, &snippets); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"Component", "SFC component", "Settings screen"} {
		source := expandAuthoringSnippet(strings.Join(snippets[name].Body, "\n"))

		component, err := Compile("example.kui", []byte(source))
		if err != nil || len(component.StyleWarnings()) != 0 {
			t.Fatalf("snippet %s: %v %v", name, err, component.StyleWarnings())
		}
	}

	source := expandAuthoringSnippet(strings.Join(snippets["Layout"].Body, "\n"))
	if _, err := parseLayout("frame.kui", []byte(source)); err != nil {
		t.Fatal(err)
	}
}

func expandAuthoringSnippet(source string) string {
	defaults := map[string]string{}
	fields := regexp.MustCompile(`\$\{([0-9]+):([^}]*)\}`)
	source = fields.ReplaceAllStringFunc(source, func(field string) string {
		match := fields.FindStringSubmatch(field)
		defaults[match[1]] = match[2]

		return match[2]
	})
	references := regexp.MustCompile(`\$\{([0-9]+)\}`)
	source = references.ReplaceAllStringFunc(source, func(field string) string { return defaults[references.FindStringSubmatch(field)[1]] })

	return strings.ReplaceAll(source, "$0", "")
}
