package uicompiler

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"go/ast"
	"io"
	"slices"
	"strings"

	"github.com/karty-game/karty-ui/schema"
)

// Layout is a build-time-only reusable panel tree. Its markup and scoped style
// are projected into each consuming component; no layout lookup ships at runtime.
type Layout struct {
	Name, Source, Style string
	styleWarnings       []Diagnostic
	styleDocument       styleSource
	markupDocument      styleSource
	root                markupNode
}

type markupNode struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Children []markupChild
	Origin   markupOrigin
}

type markupChild struct {
	Text string
	Node *markupNode
}

// Count the emitted tree, including every occurrence of a reused layout.
// Layout references and conditional/loop wrappers are not runtime elements.
type layoutBudget struct {
	elements int
}

func parseLayout(source string, data []byte) (Layout, error) {
	if len(data) > ui.MaxAssetBytes {
		return Layout{}, fmt.Errorf("%s: layout source too large: %w", source, ui.ErrTemplate)
	}

	if err := reservedMarkupError(source, string(data)); err != nil {
		return Layout{}, err
	}

	text := strings.TrimSpace(string(data))

	var styleDocument, markupDocument styleSource

	var styleWarnings []Diagnostic

	if single, ok, err := parseSingleFileComponent(source, data); ok {
		if err != nil {
			return Layout{}, err
		}

		styleWarnings = single.styleWarnings
		styleDocument = single.styleDocument
		markupDocument = single.templateDocument

		text, err = single.layoutSource(source)
		if err != nil {
			return Layout{}, err
		}

		text = strings.TrimSpace(text)
	} else if err != nil {
		return Layout{}, err
	}

	declaration, style, err := splitStyle(text)
	if err != nil {
		return Layout{}, fmt.Errorf("%s: %w", source, err)
	}

	opening := strings.IndexByte(declaration, '{')
	if !strings.HasPrefix(declaration, "layout ") || opening < 0 || !strings.HasSuffix(declaration, "}") {
		return Layout{}, fmt.Errorf("%s: expected layout Name { markup }: %w", source, ui.ErrTemplate)
	}

	name := strings.TrimSpace(declaration[len("layout "):opening])
	if !ast.IsExported(name) || strings.ContainsAny(name, " ()\t\n") {
		return Layout{}, fmt.Errorf("%s: invalid layout name %q: %w", source, name, ui.ErrTemplate)
	}

	markup := strings.TrimSpace(declaration[opening+1 : len(declaration)-1])
	if markupDocument.original == "" {
		markupDocument = sourceDocument(source, string(data), markup)
	}

	markupDocument = trimMarkupDocument(markupDocument)

	if offset := strings.IndexByte(markup, '{'); offset >= 0 {
		return Layout{}, sourceError(markupDocument.position(offset),
			"layouts cannot contain Go expressions; project page content through slots", ui.ErrTemplate)
	}

	if styleDocument.original == "" {
		styleDocument = sourceDocument(source, string(data), style)
	}

	root, err := parseMarkupTreeDocument(markup, markupDocument)
	if err != nil {
		return Layout{}, err
	}

	if root.Name.Local != "panel" {
		return Layout{}, sourceError(root.Origin.location, "layout requires one root panel", ui.ErrTemplate)
	}

	scopeLayoutNode(&root, name)
	styleDocument.scope = name

	return Layout{
		Name:           name,
		Source:         source,
		Style:          style,
		styleWarnings:  styleWarnings,
		styleDocument:  styleDocument,
		markupDocument: markupDocument,
		root:           root,
	}, nil
}

func parseMarkupTree(markup string) (markupNode, error) {
	return parseMarkupTreeDocument(markup, styleSource{text: markup, original: markup})
}

func parseMarkupTreeDocument(markup string, document styleSource) (markupNode, error) {
	cursor := 0
	decoder := xml.NewDecoder(strings.NewReader(markup))

	var roots []markupNode

	stack := make([]*markupNode, 0, 8)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			location := document.position(len(document.text))

			if _, ok := errors.AsType[*xml.SyntaxError](err); ok {
				line, _ := decoder.InputPos()
				location = styleLineLocation(document, line)
			}

			return markupNode{}, sourceError(location, "invalid markup", err)
		}

		switch value := token.(type) {
		case xml.StartElement:
			node := markupNode{
				Name:   value.Name,
				Attrs:  append([]xml.Attr(nil), value.Attr...),
				Origin: markupOrigin{location: nextMarkupLocation(document, &cursor, value.Name.Local)},
			}
			if len(stack) == 0 {
				roots = append(roots, node)
				stack = append(stack, &roots[len(roots)-1])
			} else {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, markupChild{Node: &node})
				stack = append(stack, parent.Children[len(parent.Children)-1].Node)
			}
		case xml.EndElement:
			if len(stack) == 0 || stack[len(stack)-1].Name != value.Name {
				return markupNode{}, ui.ErrTemplate
			}

			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) == 0 {
				if strings.TrimSpace(string(value)) != "" {
					return markupNode{}, ui.ErrTemplate
				}

				continue
			}

			stack[len(stack)-1].Children = append(stack[len(stack)-1].Children, markupChild{Text: string(value)})
		case xml.Comment:
		default:
			return markupNode{}, ui.ErrTemplate
		}
	}

	if len(stack) != 0 || len(roots) != 1 {
		return markupNode{}, ui.ErrTemplate
	}

	return roots[0], nil
}

func expandLayouts(source, markup string, document styleSource, layouts map[string]Layout) (string, []styleSource, []markupOrigin, error) {
	if document.original == "" {
		document = sourceDocument(source, markup, markup)
	}

	root, err := parseMarkupTreeDocument(markup, document)
	if err != nil {
		return "", nil, nil, sourceError(document.position(0), "invalid markup", err)
	}

	styles := make([]styleSource, 0, len(layouts))
	used := make(map[string]bool, len(layouts))

	expanded, err := expandNode(root, layouts, nil, used, &styles, &layoutBudget{}, 0)
	if err != nil {
		return "", nil, nil, sourceError(root.Origin.location, "invalid layout expansion", err)
	}

	origins := []markupOrigin{}

	encoded, err := encodeMarkupOrigins(expanded, &origins)
	if err != nil {
		return "", nil, nil, err
	}

	return encoded, styles, origins, nil
}

func expandNode(
	node markupNode,
	layouts map[string]Layout,
	stack []string,
	used map[string]bool,
	styles *[]styleSource,
	budget *layoutBudget,
	panelDepth int,
) (markupNode, error) {
	layout, isLayout := layouts[node.Name.Local]
	if isLayout {
		expanded, err := expandLayoutNode(node, layout, layouts, stack, used, styles, budget, panelDepth)
		if err != nil {
			return markupNode{}, sourceError(node.Origin.location, "invalid layout invocation", err)
		}

		return expanded, nil
	}

	if panelDepth > 0 && node.Name.Local != "_kartyIf" && node.Name.Local != "_kartyLoop" {
		if budget.elements >= ui.MaxElements || panelDepth > maxElementDepth {
			return markupNode{}, fmt.Errorf("expanded layout exceeds element count or depth limits: %w", ui.ErrTemplate)
		}

		budget.elements++
	}

	if isControlContainer(node.Name.Local) {
		panelDepth++
	}

	for index, child := range node.Children {
		if child.Node == nil {
			continue
		}

		expanded, err := expandNode(*child.Node, layouts, stack, used, styles, budget, panelDepth)
		if err != nil {
			return markupNode{}, err
		}

		node.Children[index].Node = &expanded
	}

	return node, nil
}

func expandLayoutNode(
	node markupNode,
	layout Layout,
	layouts map[string]Layout,
	stack []string,
	used map[string]bool,
	styles *[]styleSource,
	budget *layoutBudget,
	panelDepth int,
) (markupNode, error) {
	if !validLayoutAttributes(node.Attrs) || contains(stack, layout.Name) {
		return markupNode{}, fmt.Errorf(
			"layout %s accepts only onBack and cannot be used recursively: %w",
			layout.Name,
			ui.ErrTemplate,
		)
	}

	fills, err := layoutFills(node.Children)
	if err != nil {
		return markupNode{}, err
	}

	slots := make(map[string]bool)
	collectSlots(layout.root, slots)

	for name := range fills {
		if !slots[name] {
			return markupNode{}, fmt.Errorf("layout %s has no slot %q: %w", layout.Name, name, ui.ErrTemplate)
		}
	}

	projected, err := projectSlots(layout.root, fills)
	if err != nil {
		return markupNode{}, err
	}

	projected.Attrs = append(projected.Attrs, node.Attrs...)

	if !used[layout.Name] {
		used[layout.Name] = true
		*styles = append(*styles, layout.styleDocument)
	}

	return expandNode(projected, layouts, append(stack, layout.Name), used, styles, budget, panelDepth)
}

func validLayoutAttributes(attributes []xml.Attr) bool {
	return len(attributes) == 0 || (len(attributes) == 1 && attributes[0].Name.Space == "" && attributes[0].Name.Local == "onBack")
}

func collectSlots(node markupNode, slots map[string]bool) {
	if node.Name.Local == "slot" {
		name := ""
		if len(node.Attrs) == 1 && node.Attrs[0].Name.Local == "name" {
			name = node.Attrs[0].Value
		}

		slots[name] = true
	}

	for _, child := range node.Children {
		if child.Node != nil {
			collectSlots(*child.Node, slots)
		}
	}
}

func layoutFills(children []markupChild) (map[string][]markupChild, error) {
	fills := map[string][]markupChild{"": {}}

	for _, child := range children {
		if child.Node == nil || child.Node.Name.Local != "fragment" {
			fills[""] = append(fills[""], cloneChild(child))

			continue
		}

		if len(child.Node.Attrs) != 1 || child.Node.Attrs[0].Name.Local != "slot" {
			return nil, fmt.Errorf("fragment requires exactly one slot attribute: %w", ui.ErrTemplate)
		}

		name := child.Node.Attrs[0].Value
		if name == "" || fills[name] != nil {
			return nil, fmt.Errorf("duplicate or empty layout slot %q: %w", name, ui.ErrTemplate)
		}

		fills[name] = cloneChildren(child.Node.Children)
	}

	return fills, nil
}

func projectSlots(node markupNode, fills map[string][]markupChild) (markupNode, error) {
	// Bound projection too: a layout may repeat a large slot fill many times
	// before expansion gets a chance to count its emitted elements.
	remaining := ui.MaxElements + 1 // Includes the layout's root panel.

	return projectNode(node, fills, &remaining)
}

func projectNode(node markupNode, fills map[string][]markupChild, remaining *int) (markupNode, error) {
	if node.Name.Local != "fragment" && node.Name.Local != "_kartyIf" && node.Name.Local != "_kartyLoop" {
		if *remaining == 0 {
			return markupNode{}, fmt.Errorf("layout projection exceeds element limit: %w", ui.ErrTemplate)
		}

		*remaining--
	}

	node.Attrs = append([]xml.Attr(nil), node.Attrs...)

	children := make([]markupChild, 0, len(node.Children))
	for _, child := range node.Children {
		if child.Node == nil {
			children = append(children, cloneChild(child))

			continue
		}

		if child.Node.Name.Local != "slot" || fills == nil {
			projected, err := projectNode(*child.Node, fills, remaining)
			if err != nil {
				return markupNode{}, err
			}

			children = append(children, markupChild{Node: &projected})

			continue
		}

		fill, err := projectFill(*child.Node, fills, remaining)
		if err != nil {
			return markupNode{}, err
		}

		children = append(children, fill...)
	}

	node.Children = children

	return node, nil
}

func projectFill(slot markupNode, fills map[string][]markupChild, remaining *int) ([]markupChild, error) {
	if len(slot.Attrs) > 1 || (len(slot.Attrs) == 1 && slot.Attrs[0].Name.Local != "name") {
		return nil, fmt.Errorf("slot supports only name: %w", ui.ErrTemplate)
	}

	name := ""
	if len(slot.Attrs) == 1 {
		name = slot.Attrs[0].Value
	}

	fill, exists := fills[name]
	if !exists {
		fill = slot.Children
	}

	children := make([]markupChild, 0, len(fill))
	for _, content := range fill {
		if content.Node == nil {
			children = append(children, content)

			continue
		}

		// Fill markup belongs to the caller, so do not project its slots
		// using the containing layout's namespace.
		cloned, err := projectNode(*content.Node, nil, remaining)
		if err != nil {
			return nil, err
		}

		children = append(children, markupChild{Node: &cloned})
	}

	return children, nil
}

func encodeMarkupOrigins(root markupNode, origins *[]markupOrigin) (string, error) {
	var output bytes.Buffer

	encoder := xml.NewEncoder(&output)
	if err := encodeNode(encoder, root, origins); err != nil {
		return "", err
	}

	if err := encoder.Flush(); err != nil {
		return "", err
	}

	return output.String(), nil
}

func encodeNode(encoder *xml.Encoder, node markupNode, origins *[]markupOrigin) error {
	start := xml.StartElement{Name: node.Name, Attr: append([]xml.Attr(nil), node.Attrs...)}
	if origins != nil {
		start.Attr = append(start.Attr, originAttribute(node.Origin, origins))
	}

	if err := encoder.EncodeToken(start); err != nil {
		return err
	}

	for _, child := range node.Children {
		if child.Node != nil {
			if err := encodeNode(encoder, *child.Node, origins); err != nil {
				return err
			}
		} else if err := encoder.EncodeToken(xml.CharData(child.Text)); err != nil {
			return err
		}
	}

	return encoder.EncodeToken(start.End())
}

func cloneChildren(children []markupChild) []markupChild {
	result := make([]markupChild, len(children))
	for index, child := range children {
		result[index] = cloneChild(child)
	}

	return result
}

func cloneChild(child markupChild) markupChild {
	if child.Node == nil {
		return child
	}

	node := *child.Node
	node.Attrs = append([]xml.Attr(nil), node.Attrs...)
	node.Children = cloneChildren(node.Children)

	return markupChild{Node: &node}
}

func contains(values []string, target string) bool {
	return slices.Contains(values, target)
}
