package uicompiler

import (
	"fmt"
	"strings"

	ui "github.com/karty-game/karty-ui/schema"
)

const maxIndentedStyleDepth = 3

type indentedStyleLine struct {
	indent, number int
	text           string
	location       SourceLocation
}

type indentedStyleParser struct {
	source    string
	lines     []indentedStyleLine
	variables map[string]string
	output    *mappedStyle
	document  styleSource
	warnings  []Diagnostic
}

// lowerIndentedStyles compiles the bounded Sass-inspired authoring syntax to
// ordinary resolved style rules. Neither Sass nor variables reach the host.
func lowerIndentedStyles(source, text string) (string, error) {
	result, _, err := lowerIndentedStylesWithWarnings(source, text)

	return result, err
}

func lowerIndentedStylesWithWarnings(source, text string) (string, []Diagnostic, error) {
	document, err := lowerIndentedStyleDocument(styleSource{source: source, text: text, original: text})

	return document.text, document.warnings, err
}

func lowerIndentedStyleDocument(document styleSource) (styleSource, error) {
	if document.original == "" {
		document.original = document.text
	}

	document.lineStarts = sourceLineStarts(document.original)
	text := document.text
	parser := indentedStyleParser{source: document.source, document: document, variables: map[string]string{}, output: &mappedStyle{}}

	offset := 0
	for index, raw := range strings.Split(text, "\n") {
		location := document.position(offset + len(raw) - len(strings.TrimLeft(raw, " \t")))
		offset += len(raw) + 1
		line, _, _ := strings.Cut(raw, "//")

		line = strings.TrimRight(line, " \r")
		if strings.TrimSpace(line) == "" {
			continue
		}

		if strings.ContainsAny(line, "\t{};") {
			document.text = ""
			document.warnings = []Diagnostic{
				{
					SourceLocation: location,
					Message:        "style block ignored: use spaces and indented declarations without braces or semicolons",
				},
			}

			return document, nil
		}

		parser.lines = append(parser.lines, indentedStyleLine{
			indent: len(
				line,
			) - len(
				strings.TrimLeft(line, " "),
			),
			number:   index + 1,
			text:     strings.TrimSpace(line),
			location: location,
		})
	}

	if len(parser.lines) == 0 {
		document.text = ""

		return document, nil
	}

	base := parser.lines[0].indent
	for index := range parser.lines {
		parser.lines[index].indent -= base
	}

	index := 0
	for index < len(parser.lines) {
		line := parser.lines[index]
		if line.indent != 0 {
			return document, parser.fail(line.number, "unexpected indentation")
		}

		if strings.HasPrefix(line.text, "$") {
			if err := parser.variable(line); err != nil {
				appendStyleWarning(
					&parser.warnings,
					Diagnostic{SourceLocation: line.location, Message: fmt.Sprintf("style variable %q ignored: %v", line.text, err)},
				)
			}

			index++

			continue
		}

		if err := parser.block(&index, "", false, 0); err != nil {
			return document, err
		}

		if parser.output.Len() > ui.MaxAssetBytes {
			return document, parser.fail(line.number, "expanded styles exceed the asset limit")
		}
	}

	if parser.output.Len() > ui.MaxAssetBytes {
		return document, parser.fail(0, "expanded styles exceed the asset limit")
	}

	document.text = parser.output.String()
	document.spans = parser.output.spans
	document.warnings = parser.warnings

	return document, nil
}

func (parser *indentedStyleParser) fail(line int, message string) error {
	return sourceError(styleLineLocation(parser.document, max(1, line)), message, ui.ErrTemplate)
}

func (parser *indentedStyleParser) value(value string, line int) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "$") {
		resolved, ok := parser.variables[value[1:]]
		if !ok {
			return "", parser.fail(line, "unknown variable "+value)
		}

		return resolved, nil
	}

	if value == "" || strings.Contains(value, "$") {
		return "", parser.fail(line, "expected one literal, theme token or variable")
	}

	return value, nil
}

func (parser *indentedStyleParser) variable(line indentedStyleLine) error {
	name, value, ok := strings.Cut(line.text[1:], ":")

	name = strings.TrimSpace(name)
	if !ok || !validCSSName(name) || parser.variables[name] != "" {
		return parser.fail(line.number, "expected a unique $name: value declaration")
	}

	resolved, err := parser.value(value, line.number)
	if err != nil {
		return err
	}

	parser.variables[name] = resolved

	return nil
}

// block accepts selectors, nested &:state blocks, and one top-level media block.
// A maximum depth of three prevents recursion from depending on source size.
func (parser *indentedStyleParser) block(index *int, parent string, media bool, depth int) error {
	if depth >= maxIndentedStyleDepth {
		return parser.fail(parser.lines[*index].number, "style nesting is too deep")
	}

	line := parser.lines[*index]
	header := line.text
	isMedia := strings.HasPrefix(header, "@media")

	header = indentedStyleHeader(header, parent)

	*index++
	if *index >= len(parser.lines) || parser.lines[*index].indent <= line.indent {
		return parser.fail(line.number, "expected an indented block")
	}

	childIndent := parser.lines[*index].indent
	if isMedia {
		parser.output.write(header+" {\n", line.location)
	}

	var (
		declarations mappedStyle
		nested       mappedStyle
	)

	for *index < len(parser.lines) && parser.lines[*index].indent > line.indent {
		child := parser.lines[*index]
		if child.indent != childIndent {
			return parser.fail(child.number, "inconsistent indentation")
		}

		if !isMedia && parser.hasNestedBlock(*index) {
			used := declarations.Len() + nested.Len() + parser.output.Len()

			resolved, err := parser.nestedBlock(index, header, media, depth+1, used)
			if err != nil {
				return err
			}

			nested.append(resolved)

			continue
		}

		if isMedia {
			if err := parser.block(index, "", true, depth+1); err != nil {
				return err
			}

			continue
		}

		name, raw, ok := strings.Cut(child.text, ":")

		name = canonicalStyleProperty(strings.TrimSpace(name))
		if !ok || !validCSSName(name) {
			declarations.write(child.text+";\n", child.location)

			*index++

			continue
		}

		value, err := parser.value(raw, child.number)
		if err != nil {
			value = strings.TrimSpace(raw)
		}

		declarations.write(name+": "+value+";\n", child.location)

		if declarations.Len()+nested.Len()+parser.output.Len() > ui.MaxAssetBytes {
			return parser.fail(child.number, "expanded styles exceed the asset limit")
		}

		*index++
	}

	if isMedia {
		parser.output.write("}\n", line.location)
	} else {
		parser.output.write(header+" {\n", line.location)
		parser.output.append(&declarations)
		parser.output.write("}\n", line.location)
		parser.output.append(&nested)
	}

	if parser.output.Len() > ui.MaxAssetBytes {
		return parser.fail(line.number, "expanded styles exceed the asset limit")
	}

	return nil
}

func canonicalStyleProperty(property string) string {
	switch property {
	case "direction":
		return "flex-direction"
	case "align":
		return "align-items"
	case "justify":
		return "justify-content"
	case "grow":
		return "flex-grow"
	default:
		return property
	}
}

func (parser *indentedStyleParser) hasNestedBlock(index int) bool {
	return strings.HasPrefix(parser.lines[index].text, "&:") ||
		(index+1 < len(parser.lines) && parser.lines[index+1].indent > parser.lines[index].indent)
}

func indentedStyleHeader(header, parent string) string {
	if parent == "" {
		return header
	}

	if strings.HasPrefix(header, "&:") {
		return parent + header[1:]
	}

	return parent + " " + header
}

func (parser *indentedStyleParser) nestedBlock(index *int, parent string, media bool, depth, used int) (*mappedStyle, error) {
	line := parser.lines[*index].number
	saved := parser.output
	parser.output = &mappedStyle{}
	err := parser.block(index, parent, media, depth)
	resolved := parser.output
	parser.output = saved

	if err != nil {
		return nil, err
	}

	if used+resolved.Len() > ui.MaxAssetBytes {
		return nil, parser.fail(line, "expanded styles exceed the asset limit")
	}

	return resolved, nil
}
