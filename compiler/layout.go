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
	root                markupNode
}

type markupNode struct {
	Name     xml.Name
	Attrs    []xml.Attr
	Children []markupChild
}

type markupChild struct {
	Text string
	Node *markupNode
}

func parseLayout(source string, data []byte) (Layout, error) {
	if len(data) > ui.MaxAssetBytes {
		return Layout{}, fmt.Errorf("%s: layout source too large: %w", source, ui.ErrTemplate)
	}

	declaration, style, err := splitStyle(strings.TrimSpace(string(data)))
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
	if strings.Contains(markup, "{") {
		return Layout{}, fmt.Errorf(
			"%s: layouts cannot contain Go expressions; project page content through slots: %w",
			source,
			ui.ErrTemplate,
		)
	}

	root, err := parseMarkupTree(markup)
	if err != nil || root.Name.Local != "panel" {
		return Layout{}, fmt.Errorf("%s: layout requires one root panel: %w", source, ui.ErrTemplate)
	}

	return Layout{Name: name, Source: source, Style: style, root: root}, nil
}

func parseMarkupTree(markup string) (markupNode, error) {
	decoder := xml.NewDecoder(strings.NewReader(markup))

	var roots []markupNode

	stack := make([]*markupNode, 0, 8)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return markupNode{}, err
		}

		switch value := token.(type) {
		case xml.StartElement:
			node := markupNode{Name: value.Name, Attrs: append([]xml.Attr(nil), value.Attr...)}
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

func expandLayouts(source, markup string, layouts map[string]Layout) (string, []string, error) {
	if len(layouts) == 0 {
		return markup, nil, nil
	}

	root, err := parseMarkupTree(markup)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", source, err)
	}

	styles := make([]string, 0, len(layouts))
	used := make(map[string]bool, len(layouts))

	expanded, err := expandNode(root, layouts, nil, used, &styles)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", source, err)
	}

	encoded, err := encodeMarkup(expanded)
	if err != nil {
		return "", nil, err
	}

	return encoded, styles, nil
}

func expandNode(
	node markupNode,
	layouts map[string]Layout,
	stack []string,
	used map[string]bool,
	styles *[]string,
) (markupNode, error) {
	layout, isLayout := layouts[node.Name.Local]
	if isLayout {
		return expandLayoutNode(node, layout, layouts, stack, used, styles)
	}

	for index, child := range node.Children {
		if child.Node == nil {
			continue
		}

		expanded, err := expandNode(*child.Node, layouts, stack, used, styles)
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
	styles *[]string,
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
		*styles = append(*styles, layout.Style)
	}

	return expandNode(projected, layouts, append(stack, layout.Name), used, styles)
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
	children := make([]markupChild, 0, len(node.Children))
	for _, child := range node.Children {
		if child.Node == nil {
			children = append(children, cloneChild(child))

			continue
		}

		if child.Node.Name.Local != "slot" {
			projected, err := projectSlots(*child.Node, fills)
			if err != nil {
				return markupNode{}, err
			}

			children = append(children, markupChild{Node: &projected})

			continue
		}

		name := ""

		if len(child.Node.Attrs) > 1 || (len(child.Node.Attrs) == 1 && child.Node.Attrs[0].Name.Local != "name") {
			return markupNode{}, fmt.Errorf("slot supports only name: %w", ui.ErrTemplate)
		}

		if len(child.Node.Attrs) == 1 {
			name = child.Node.Attrs[0].Value
		}

		fill, exists := fills[name]
		if !exists {
			fill = child.Node.Children
		}

		children = append(children, cloneChildren(fill)...)
	}

	node.Children = children

	return node, nil
}

func encodeMarkup(root markupNode) (string, error) {
	var output bytes.Buffer

	encoder := xml.NewEncoder(&output)
	if err := encodeNode(encoder, root); err != nil {
		return "", err
	}

	if err := encoder.Flush(); err != nil {
		return "", err
	}

	return output.String(), nil
}

func encodeNode(encoder *xml.Encoder, node markupNode) error {
	start := xml.StartElement{Name: node.Name, Attr: node.Attrs}
	if err := encoder.EncodeToken(start); err != nil {
		return err
	}

	for _, child := range node.Children {
		if child.Node != nil {
			if err := encodeNode(encoder, *child.Node); err != nil {
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
