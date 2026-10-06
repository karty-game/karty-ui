package uicompiler

import (
	"encoding/xml"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strconv"
	"strings"

	"github.com/karty-game/karty-ui/schema"
)

var loopLine = regexp.MustCompile(`(?m)^[\t ]*for ([^\n]+)\{[\t ]*$`)
var conditionLine = regexp.MustCompile(`(?m)^[\t ]*if ([^\n]+)\{[\t ]*$`)

func (component *Component) lowerConditions(body string) (string, error) {
	if strings.Contains(body, "_kartyIf") {
		return "", ui.ErrTemplate
	}

	var output strings.Builder

	for {
		match := conditionLine.FindStringSubmatchIndex(body)
		if match == nil {
			output.WriteString(body)

			break
		}

		condition := strings.TrimSpace(body[match[2]:match[3]])
		if _, err := parser.ParseExpr(condition); err != nil {
			return "", &expressionError{fragment: body[match[0]:match[1]], cause: err}
		}

		opening := strings.LastIndex(body[:match[1]], "{")

		end, err := expressionEnd(body[opening:])
		if err != nil {
			return "", &expressionError{fragment: body[match[0]:match[1]], cause: err}
		}

		inside := body[opening+1 : opening+end]
		if conditionLine.MatchString(inside) {
			return "", &expressionError{
				fragment: body[match[0]:match[1]],
				cause:    fmt.Errorf("nest conditions through child components: %w", ui.ErrTemplate),
			}
		}

		index := len(component.conditions)
		component.Conditional = true
		component.conditions = append(component.conditions, condition)

		output.WriteString(body[:match[0]])
		fmt.Fprintf(&output, "<_kartyIf index=\"%d\">%s</_kartyIf>", index, inside)

		remainder := body[opening+end+1:]

		trimmed := strings.TrimLeft(remainder, " \t\r\n")
		if strings.HasPrefix(trimmed, "else {") {
			elseOpening := strings.IndexByte(trimmed, '{')

			elseEnd, elseErr := expressionEnd(trimmed[elseOpening:])
			if elseErr != nil {
				return "", &expressionError{fragment: "else {", cause: elseErr}
			}

			output.WriteString(strings.Repeat("\n", strings.Count(remainder[:len(remainder)-len(trimmed)], "\n")))

			elseIndex := len(component.conditions)
			component.conditions = append(component.conditions, "!("+condition+")")

			fmt.Fprintf(&output, "<_kartyIf index=\"%d\">%s</_kartyIf>", elseIndex,
				trimmed[elseOpening+1:elseOpening+elseEnd])
			remainder = trimmed[elseOpening+elseEnd+1:]
		}

		body = remainder
	}

	return output.String(), nil
}

func (component *Component) lowerLoops(body string) (string, error) {
	if strings.Contains(body, "_kartyLoop") {
		return "", ui.ErrTemplate
	}

	var output strings.Builder

	for {
		match := loopLine.FindStringSubmatchIndex(body)
		if match == nil {
			output.WriteString(body)

			break
		}

		header := strings.TrimSpace(body[match[2]:match[3]])

		parsed, err := parser.ParseFile(token.NewFileSet(), component.Source, "package main\nfunc f(){ for "+header+" {} }", 0)
		if err != nil {
			return "", &expressionError{fragment: body[match[0]:match[1]], cause: err}
		}

		statement, ok := parsed.Decls[0].(*ast.FuncDecl).Body.List[0].(*ast.RangeStmt)
		if !ok || statement.Tok != token.DEFINE {
			return "", &expressionError{
				fragment: body[match[0]:match[1]],
				cause:    fmt.Errorf("expected for ... := range expression: %w", ui.ErrTemplate),
			}
		}

		opening := strings.LastIndex(body[:match[1]], "{")

		end, err := expressionEnd(body[opening:])
		if err != nil {
			return "", &expressionError{fragment: body[match[0]:match[1]], cause: err}
		}

		inside := body[opening+1 : opening+end]
		if loopLine.MatchString(inside) {
			return "", &expressionError{
				fragment: body[match[0]:match[1]],
				cause:    fmt.Errorf("nest loops through child components: %w", ui.ErrTemplate),
			}
		}

		index := len(component.loops)
		component.loops = append(component.loops, "for "+header)

		output.WriteString(body[:match[0]])
		fmt.Fprintf(&output, "<_kartyLoop index=\"%d\">%s</_kartyLoop>", index, inside)

		body = body[opening+end+1:]
	}

	return output.String(), nil
}

func (component *Component) addChild(node xml.StartElement, expressions map[string]string) error {
	if len(component.Bindings) >= ui.MaxElements || len(component.parents) >= maxElementDepth {
		return fmt.Errorf("element count or depth exceeds template limits: %w", ui.ErrTemplate)
	}

	binding := Binding{
		ID: uint32(len(component.Bindings) + 1), Enabled: "true", Visible: component.activeCondition,
		Child: node.Name.Local, Loop: component.activeLoop,
	}
	for _, attribute := range node.Attr {
		value, exists := expressions[attribute.Value]
		if !exists || attribute.Name.Space != "" {
			return ui.ErrTemplate
		}

		switch attribute.Name.Local {
		case "props":
			if binding.Props != "" {
				return ui.ErrTemplate
			}

			binding.Props = value
		case "key":
			if binding.Key != "" {
				return ui.ErrTemplate
			}

			binding.Key = value
		default:
			return fmt.Errorf("child %s supports only props/key: %w", binding.Child, ui.ErrTemplate)
		}
	}

	if binding.Props == "" || (binding.Loop != "" && binding.Key == "") {
		return fmt.Errorf("child requires props and loop children require key: %w", ui.ErrTemplate)
	}

	if binding.Key == "" {
		binding.Key = "uint32(0)"
	}

	component.Local, component.Composition = true, true
	component.Template.Version = max(component.Template.Version, ui.SchemaComposition)
	component.Template.Elements = append(
		component.Template.Elements,
		ui.Element{
			ID: binding.ID, Parent: component.parent,
			Name: "element_" + strconv.Itoa(int(binding.ID)), Kind: "slot",
		},
	)
	component.Bindings = append(component.Bindings, binding)
	component.elementClasses = append(component.elementClasses, "")

	return nil
}

func resolveChildren(components []Component) error {
	names := map[string]int{}
	for index, component := range components {
		names[component.Name] = index
	}

	visiting, done := map[string]bool{}, map[string]bool{}
	requiresInteraction := make([]bool, len(components))

	var visit func(int) error

	visit = func(index int) error {
		component := &components[index]
		if visiting[component.Name] {
			return fmt.Errorf("%s: recursive child composition: %w", component.Source, ui.ErrTemplate)
		}

		if done[component.Name] {
			return nil
		}

		visiting[component.Name] = true
		for _, element := range component.Template.Elements {
			requiresInteraction[index] = requiresInteraction[index] || element.Action != 0
		}

		for _, binding := range component.Bindings {
			if binding.Child == "" {
				continue
			}

			child, exists := names[binding.Child]
			if !exists {
				return fmt.Errorf("%s: unknown child %s: %w", component.Source, binding.Child, ui.ErrTemplate)
			}

			target := &components[child]
			if len(target.Parameters) != 1 {
				return fmt.Errorf("%s: child must accept one typed props argument: %w", target.Source, ui.ErrTemplate)
			}

			target.Local, target.Composition, target.ChildFactory = true, true, true

			if err := visit(child); err != nil {
				return err
			}

			requiresInteraction[index] = requiresInteraction[index] || requiresInteraction[child]
		}

		// The host uses the root schema to permit actions throughout its tree.
		if !component.Template.Modal && requiresInteraction[index] {
			component.Template.Version = max(component.Template.Version, ui.SchemaInteractionPolish)
		}

		visiting[component.Name] = false
		done[component.Name] = true

		return nil
	}
	for index := range components {
		if err := visit(index); err != nil {
			return err
		}
	}

	return nil
}
