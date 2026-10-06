package uicompiler

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"path/filepath"
	"strings"
	"unicode"

	ui "github.com/karty-game/karty-ui/schema"
)

type singleFileComponent struct {
	script           string
	template         string
	style            string
	preamble, setup  string
	parameters       []Parameter
	hasScript        bool
	styleWarnings    []Diagnostic
	styleDocument    styleSource
	templateDocument styleSource
	scriptStart      int
}

// parseSingleFileComponent parses the .kui single-file syntax. Script
// content is a Go file containing imports/types and one setup function.
func parseSingleFileComponent(source string, data []byte) (singleFileComponent, error) {
	original := string(data)
	if !strings.HasSuffix(source, ".kui") {
		return singleFileComponent{}, sourceError(sourcePosition(source, original, 0), "use a .kui single-file component", ui.ErrTemplate)
	}

	if !strings.HasPrefix(strings.TrimSpace(original), "<") {
		return singleFileComponent{}, sourceError(
			sourcePosition(source, original, 0),
			"expected a .kui <template> block; legacy declarations are unsupported",
			ui.ErrTemplate,
		)
	}

	result := singleFileComponent{}
	seen := map[string]bool{}

	text := original
	for len(text) > 0 {
		text = strings.TrimLeft(text, " \t\r\n")
		if text == "" {
			break
		}

		offset := len(original) - len(text)
		location := sourcePosition(source, original, offset)

		if strings.HasPrefix(text, "<!--") {
			end := strings.Index(text, "-->")
			if end < 0 {
				return result, sourceError(location, "unclosed SFC comment", ui.ErrTemplate)
			}

			text = text[end+len("-->"):]

			continue
		}

		openingEnd := sfcOpeningEnd(text)
		if openingEnd < 0 {
			return result, sourceError(location, "expected an opening SFC block", ui.ErrTemplate)
		}

		name, _, err := sfcOpening(text[:openingEnd+1])
		if err != nil {
			return result, sourceError(location, "invalid SFC opening block", err)
		}

		if seen[name] {
			return result, sourceError(location, "duplicate SFC "+name+" block", ui.ErrTemplate)
		}

		seen[name] = true
		contentStart := offset + openingEnd + 1

		closeAt, closeEnd := sfcClosing(text[openingEnd+1:], name)
		if closeAt < 0 {
			return result, sourceError(location, "missing </"+name+">", ui.ErrTemplate)
		}

		content := text[openingEnd+1 : openingEnd+1+closeAt]
		document := styleSource{source: source, text: content, original: original, start: contentStart}

		switch name {
		case "script":
			result.script = content
			result.scriptStart = contentStart
		case "template":
			result.template = content
			result.templateDocument = document
		case "style":
			result.style = content
			result.styleDocument = document
		}

		text = text[openingEnd+1+closeEnd:]
	}

	return finishSingleFileBlocks(source, original, result, seen["template"], seen["script"])
}

func finishSingleFileBlocks(source, original string, result singleFileComponent, hasTemplate, hasScript bool) (singleFileComponent, error) {
	if !hasTemplate || strings.TrimSpace(result.template) == "" {
		return result, sourceError(sourcePosition(source, original, 0), "SFC requires a non-empty template block", ui.ErrTemplate)
	}

	if strings.TrimSpace(result.style) != "" {
		document, err := lowerIndentedStyleDocument(result.styleDocument)
		if err != nil {
			return result, err
		}

		result.styleDocument = document
		result.style = document.text
		result.styleWarnings = document.warnings
	}

	if hasScript {
		var err error

		result.preamble, result.parameters, result.setup, err = sfcSetup(source, result.script)
		if err != nil {
			if failures, ok := errors.AsType[scanner.ErrorList](err); ok {
				failure := failures[0]

				return result, sourceError(
					sourcePosition(source, original, result.scriptStart+max(0, failure.Pos.Offset-len("package kartui\n"))),
					failure.Msg,
					ui.ErrTemplate,
				)
			}

			return result, sourceError(sourcePosition(source, original, result.scriptStart), "invalid Go setup", err)
		}

		result.hasScript = true
	}

	return result, nil
}

// sfcSetup returns Go package declarations, the setup signature parameters,
// and the setup body. The source remains valid Go for editor tooling.
func sfcSetup(source, script string) (preamble string, parameters []Parameter, body string, err error) {
	const packagePrefix = "package kartui\n"

	set := token.NewFileSet()

	file, parseErr := parser.ParseFile(set, source, packagePrefix+script, parser.ParseComments)
	if parseErr != nil {
		return "", nil, "", parseErr
	}

	if len(file.Decls) == 0 {
		return "", nil, "", fmt.Errorf("%s: Go script requires one setup function: %w", source, ui.ErrTemplate)
	}

	var setup *ast.FuncDecl

	for index, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			if value.Tok != token.IMPORT && value.Tok != token.TYPE {
				return "", nil, "", fmt.Errorf("%s: Go script supports imports, types, and setup: %w", source, ui.ErrTemplate)
			}
		case *ast.FuncDecl:
			if setup != nil || value.Name.Name != "setup" || value.Recv != nil || value.Type.Results != nil ||
				value.Type.TypeParams != nil || value.Body == nil || index != len(file.Decls)-1 {
				return "", nil, "", fmt.Errorf(
					"%s: Go script requires one final func setup(args) with no result: %w",
					source,
					ui.ErrTemplate,
				)
			}

			setup = value
		default:
			return "", nil, "", fmt.Errorf("%s: unsupported Go script declaration: %w", source, ui.ErrTemplate)
		}
	}

	if setup == nil {
		return "", nil, "", fmt.Errorf("%s: Go script requires one setup function: %w", source, ui.ErrTemplate)
	}

	if setup.Type.Params == nil {
		return "", nil, "", fmt.Errorf("%s: setup function has no parameter list: %w", source, ui.ErrTemplate)
	}

	params, err := setupParameters(source, setup.Type.Params)
	if err != nil {
		return "", nil, "", err
	}

	fileStart := len(packagePrefix)
	setupStart := set.Position(setup.Pos()).Offset - fileStart
	bodyStart := set.Position(setup.Body.Lbrace).Offset - fileStart + 1

	bodyEnd := set.Position(setup.Body.Rbrace).Offset - fileStart
	if setupStart < 0 || bodyStart < 0 || bodyEnd < bodyStart || bodyEnd > len(script) {
		return "", nil, "", fmt.Errorf("%s: invalid Go setup positions: %w", source, ui.ErrTemplate)
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

func setupParameters(source string, fields *ast.FieldList) ([]Parameter, error) {
	var parameters []Parameter

	seen := map[string]bool{}

	for _, field := range fields.List {
		var output bytes.Buffer
		if err := format.Node(&output, token.NewFileSet(), field.Type); err != nil {
			return nil, err
		}

		kind := output.String()

		if len(field.Names) == 0 {
			return nil, fmt.Errorf("%s: parameters must be named: %w", source, ui.ErrTemplate)
		}

		for _, name := range field.Names {
			if seen[name.Name] || name.Name == "invalidate" ||
				strings.HasPrefix(name.Name, "_karty") {
				return nil, fmt.Errorf("%s: parameter %s must be unique and not reserved: %w", source, name, ui.ErrTemplate)
			}

			seen[name.Name] = true
			parameters = append(parameters, Parameter{Name: name.Name, Type: kind})
		}
	}

	return parameters, nil
}
