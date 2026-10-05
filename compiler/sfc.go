package uicompiler

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"unicode"

	ui "github.com/karty-game/karty-ui/schema"
)

type singleFileComponent struct {
	script   string
	template string
	style    string
}

// parseSingleFileComponent recognizes the experimental SFC syntax. Script
// content is a Go file containing imports/types and one setup function.
func parseSingleFileComponent(source string, data []byte) (singleFileComponent, bool, error) {
	if !strings.HasPrefix(strings.TrimSpace(string(data)), "<") {
		return singleFileComponent{}, false, nil
	}

	text := strings.TrimSpace(string(data))
	result := singleFileComponent{}
	seen := map[string]bool{}

	for len(text) > 0 {
		text = strings.TrimLeft(text, " \t\r\n")
		if text == "" {
			break
		}

		if !strings.HasPrefix(text, "<") {
			return result, true, fmt.Errorf("%s: expected an SFC block: %w", source, ui.ErrTemplate)
		}

		openingEnd := strings.IndexByte(text, '>')
		if openingEnd < 0 {
			return result, true, fmt.Errorf("%s: unclosed SFC block: %w", source, ui.ErrTemplate)
		}

		opening := text[:openingEnd+1]

		var name string

		switch opening {
		case `<script setup lang="go">`:
			name = "script"
		case "<template>":
			name = "template"
		case "<style>":
			name = "style"
		default:
			return result, true, fmt.Errorf("%s: unsupported SFC block %s: %w", source, opening, ui.ErrTemplate)
		}

		if seen[name] {
			return result, true, fmt.Errorf("%s: duplicate SFC %s block: %w", source, name, ui.ErrTemplate)
		}

		seen[name] = true

		closing := "</" + name + ">"

		closeAt := strings.Index(text[openingEnd+1:], closing)
		if closeAt < 0 {
			return result, true, fmt.Errorf("%s: missing %s: %w", source, closing, ui.ErrTemplate)
		}

		contentStart := openingEnd + 1
		contentEnd := contentStart + closeAt
		content := text[contentStart:contentEnd]

		switch name {
		case "script":
			result.script = content
		case "template":
			result.template = content
		case "style":
			result.style = content
		}

		text = text[contentEnd+len(closing):]
	}

	if !seen["template"] || strings.TrimSpace(result.template) == "" {
		return result, true, fmt.Errorf("%s: SFC requires a non-empty template block: %w", source, ui.ErrTemplate)
	}

	if seen["script"] {
		if _, _, _, err := sfcSetup(source, result.script); err != nil {
			return result, true, err
		}
	}

	return result, true, nil
}

func (component singleFileComponent) componentSource(source string) (string, error) {
	name, err := inferredComponentName(source)
	if err != nil {
		return "", err
	}

	preamble, parameters, body := "", "()", ""
	if component.script != "" {
		preamble, parameters, body, err = sfcSetup(source, component.script)
		if err != nil {
			return "", err
		}
	}

	var result strings.Builder
	if preamble != "" {
		result.WriteString(preamble)
		result.WriteByte('\n')
	}

	if component.script != "" {
		result.WriteString("setup ")
		result.WriteString(name)
		result.WriteString(parameters)
		result.WriteString(" {\n")
		result.WriteString(body)
		result.WriteString("\n}\n")
	}

	result.WriteString("kartui ")
	result.WriteString(name)

	if component.script == "" {
		result.WriteString(parameters)
	}

	result.WriteString(" {\n")
	result.WriteString(component.template)
	result.WriteString("\n}\n")

	if component.style != "" {
		result.WriteString("style {\n")
		result.WriteString(component.style)
		result.WriteString("\n}\n")
	}

	return result.String(), nil
}

func (component singleFileComponent) layoutSource(source string) (string, error) {
	name, err := inferredComponentName(source)
	if err != nil {
		return "", err
	}

	if component.script != "" {
		return "", fmt.Errorf("%s: layout SFC cannot contain a script block: %w", source, ui.ErrTemplate)
	}

	var result strings.Builder
	result.WriteString("layout ")
	result.WriteString(name)
	result.WriteString(" {\n")
	result.WriteString(component.template)
	result.WriteString("\n}\n")

	if component.style != "" {
		result.WriteString("style {\n")
		result.WriteString(component.style)
		result.WriteString("\n}\n")
	}

	return result.String(), nil
}

// sfcSetup returns Go package declarations, the setup signature parameters,
// and the setup body. The source remains valid Go for editor tooling.
func sfcSetup(source, script string) (preamble, parameters, body string, err error) {
	const packagePrefix = "package kartui\n"

	set := token.NewFileSet()

	file, parseErr := parser.ParseFile(set, source, packagePrefix+script, parser.ParseComments)
	if parseErr != nil {
		return "", "", "", parseErr
	}

	if len(file.Decls) == 0 {
		return "", "", "", fmt.Errorf("%s: Go script requires one setup function: %w", source, ui.ErrTemplate)
	}

	var setup *ast.FuncDecl

	for index, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			if value.Tok != token.IMPORT && value.Tok != token.TYPE {
				return "", "", "", fmt.Errorf("%s: Go script supports imports, types, and setup: %w", source, ui.ErrTemplate)
			}
		case *ast.FuncDecl:
			if setup != nil || value.Name.Name != "setup" || value.Recv != nil || value.Type.Results != nil ||
				value.Type.TypeParams != nil || value.Body == nil || index != len(file.Decls)-1 {
				return "", "", "", fmt.Errorf(
					"%s: Go script requires one final func setup(args) with no result: %w",
					source,
					ui.ErrTemplate,
				)
			}

			setup = value
		default:
			return "", "", "", fmt.Errorf("%s: unsupported Go script declaration: %w", source, ui.ErrTemplate)
		}
	}

	if setup == nil {
		return "", "", "", fmt.Errorf("%s: Go script requires one setup function: %w", source, ui.ErrTemplate)
	}

	if setup.Type.Params == nil {
		return "", "", "", fmt.Errorf("%s: setup function has no parameter list: %w", source, ui.ErrTemplate)
	}

	var params string

	paramsStart := set.Position(setup.Type.Params.Pos()).Offset - len(packagePrefix)

	paramsEnd := set.Position(setup.Type.Params.End()).Offset - len(packagePrefix)
	switch {
	case len(setup.Type.Params.List) == 0:
		params = "()"
	case paramsStart < 0 || paramsEnd < paramsStart || paramsEnd > len(script):
		return "", "", "", fmt.Errorf("%s: invalid Go setup parameters: %w", source, ui.ErrTemplate)
	default:
		params = script[paramsStart:paramsEnd]
	}

	fileStart := len(packagePrefix)
	setupStart := set.Position(setup.Pos()).Offset - fileStart
	bodyStart := set.Position(setup.Body.Lbrace).Offset - fileStart + 1

	bodyEnd := set.Position(setup.Body.Rbrace).Offset - fileStart
	if setupStart < 0 || bodyStart < 0 || bodyEnd < bodyStart || bodyEnd > len(script) {
		return "", "", "", fmt.Errorf("%s: invalid Go setup positions: %w", source, ui.ErrTemplate)
	}

	return strings.TrimSpace(script[:setupStart]), params, script[bodyStart:bodyEnd], nil
}

func inferredComponentName(source string) (string, error) {
	base := strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))

	parts := strings.FieldsFunc(base, func(value rune) bool {
		return value == '-' || value == '_' || value == ' ' || value == '.'
	})
	if len(parts) == 0 {
		return "", fmt.Errorf("%s: filename cannot produce a component name: %w", source, ui.ErrTemplate)
	}

	var name strings.Builder

	for _, part := range parts {
		first, size := utf8FirstRune(part)
		name.WriteRune(unicode.ToUpper(first))
		name.WriteString(part[size:])
	}

	result := name.String()
	if !token.IsIdentifier(result) || !ast.IsExported(result) {
		return "", fmt.Errorf("%s: filename produces invalid component name %q: %w", source, result, ui.ErrTemplate)
	}

	return result, nil
}

func utf8FirstRune(text string) (rune, int) {
	for index, value := range text {
		return value, index + len(string(value))
	}

	return 0, 0
}
