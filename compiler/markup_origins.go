package uicompiler

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"

	ui "github.com/karty-game/karty-ui/schema"
)

const markupOriginAttribute = "_kartyOrigin"
const responsiveIndexOffset = 2 // Root warnings use 0/-1; child warnings use 1/-2 onwards.

type markupOrigin struct {
	scope    string
	location SourceLocation
}

// Locate actual markup in authored text, skipping comments and Go expressions.
// The lowerer may insert conditional wrappers or move projected slot contents;
// storing origins on nodes keeps those edits from changing source attribution.
func nextMarkupLocation(document styleSource, cursor *int, name string) SourceLocation {
	if strings.HasPrefix(name, "_karty") {
		return document.position(*cursor)
	}

	text := document.text
	for *cursor < len(text) {
		offset := *cursor
		*cursor++

		if strings.HasPrefix(text[offset:], "<!--") {
			end := strings.Index(text[offset:], "-->")
			if end >= 0 {
				*cursor = offset + end + len("-->")
			}

			continue
		}

		if text[offset] == '{' {
			start := strings.LastIndexByte(text[:offset], '\n') + 1

			prefix := strings.TrimSpace(text[start:offset])
			if strings.HasPrefix(prefix, "if ") || strings.HasPrefix(prefix, "for ") || strings.HasSuffix(prefix, "else") {
				continue
			}

			if end, err := expressionEnd(text[offset:]); err == nil {
				*cursor = offset + end + 1
			}

			continue
		}

		prefix := "<" + name
		if strings.HasPrefix(text[offset:], prefix) {
			end := offset + len(prefix)
			if end < len(text) && strings.ContainsRune(" \t\r\n>/", rune(text[end])) {
				return document.position(offset)
			}
		}
	}

	return document.position(0)
}

func scopeLayoutNode(node *markupNode, scope string) {
	node.Origin.scope = scope
	for _, child := range node.Children {
		if child.Node != nil {
			scopeLayoutNode(child.Node, scope)
		}
	}
}

func (component *Component) consumeOrigin(node xml.StartElement) (xml.StartElement, markupOrigin, error) {
	origin := markupOrigin{location: SourceLocation{Source: component.Source, Line: 1, Column: 1}}

	for index, attribute := range node.Attr {
		if attribute.Name.Local != markupOriginAttribute {
			continue
		}

		identifier, err := strconv.Atoi(attribute.Value)
		if err != nil || identifier < 0 || identifier >= len(component.origins) {
			return node, origin, ui.ErrTemplate
		}

		origin = component.origins[identifier]

		node.Attr = append(node.Attr[:index:index], node.Attr[index+1:]...)

		return node, origin, nil
	}

	return node, origin, nil
}

func originAttribute(origin markupOrigin, origins *[]markupOrigin) xml.Attr {
	identifier := len(*origins)
	*origins = append(*origins, origin)

	return xml.Attr{Name: xml.Name{Local: markupOriginAttribute}, Value: strconv.Itoa(identifier)}
}

func sourceDocument(source, original, text string) styleSource {
	start := max(strings.Index(original, text), 0)

	return styleSource{source: source, original: original, text: text, start: start}
}

func trimMarkupDocument(document styleSource) styleSource {
	trimmed := strings.TrimSpace(document.text)
	document.start += len(document.text) - len(strings.TrimLeft(document.text, " \t\r\n"))
	document.text = trimmed
	document.lineStarts = sourceLineStarts(document.original)

	return document
}

func reservedMarkupError(source, text string) error {
	if strings.Contains(text, markupOriginAttribute) {
		return fmt.Errorf("%s: %s is reserved compiler metadata: %w", source, markupOriginAttribute, ui.ErrTemplate)
	}

	return nil
}

func (component *Component) startLocatedComposition(
	node xml.StartElement,
	depth int,
	origin markupOrigin,
	expressions map[string]string,
) error {
	before := len(component.Template.Elements)
	if err := component.startComposition(node, depth, expressions); err != nil {
		return sourceError(origin.location, "invalid "+node.Name.Local+" element", err)
	}

	if depth == 1 {
		component.panelScope = origin.scope
		component.panelLocation = origin.location
	}

	for len(component.elementScopes) < len(component.Template.Elements) && len(component.Template.Elements) > before {
		component.elementScopes = append(component.elementScopes, origin.scope)
		component.elementLocations = append(component.elementLocations, origin.location)
	}

	return nil
}
