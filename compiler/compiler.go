// Package uicompiler lowers KartUI UI sources into presentation assets
// and typed client bindings. It never evaluates user Go expressions during build.
package uicompiler

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io"
	"strconv"
	"strings"

	"github.com/karty-game/karty-ui/schema"
)

const maxPanelAttributes = 3

type Parameter struct{ Name, Type string }
type Binding struct {
	ID                                               uint32
	Text, DefaultText, Enabled, Visible, Rows, Click string
	Child, Props, Key, Loop                          string
}
type Component struct {
	Name, Source, Asset                    string
	Local                                  bool
	Composition, ChildFactory, Conditional bool
	loops                                  []string
	activeLoop                             string
	conditions                             []string
	activeCondition                        string
	conditionDepth                         int
	Setup, Preamble                        string
	Back                                   string
	hasSetup                               bool
	setupSignature                         string
	Parameters                             []Parameter
	Bindings                               []Binding
	Template                               ui.Template
	panelClass                             string
	elementClasses                         []string
	parent                                 uint32
	parents                                []uint32
	loopDepth                              int
}

// Compile accepts one KartUI component with optional Go imports/types and setup.
// Legacy exported scalar props remain supported; local models compile with the client.
func Compile(source string, data []byte) (Component, error) {
	return CompileWithTheme(source, data, DefaultTheme())
}

// CompileWithTheme resolves style tokens into the bounded presentation asset.
func CompileWithTheme(source string, data []byte, theme Theme) (Component, error) {
	return compileWithLayouts(source, data, theme, nil)
}

//nolint:gocognit,gocyclo // One ordered compilation pipeline keeps source attribution and validation together.
func compileWithLayouts(source string, data []byte, theme Theme, layouts map[string]Layout) (Component, error) {
	result := Component{Source: source, Template: ui.Template{Version: 1}}
	if len(data) > ui.MaxAssetBytes {
		return result, fmt.Errorf("%s: source too large: %w", source, ui.ErrTemplate)
	}

	text := strings.TrimSpace(string(data))
	if single, ok, err := parseSingleFileComponent(source, data); ok {
		if err != nil {
			return result, err
		}

		text, err = single.componentSource(source)
		if err != nil {
			return result, err
		}

		text = strings.TrimSpace(text)
	} else if err != nil {
		return result, err
	}

	text, styleText, err := splitStyle(text)
	if err != nil {
		return result, fmt.Errorf("%s: %w", source, err)
	}

	text, err = result.preamble(source, text)
	if err != nil {
		return result, err
	}

	opening := strings.IndexByte(text, '{')
	if !strings.HasPrefix(text, "kartui ") || opening < 0 || !strings.HasSuffix(text, "}") {
		return result, fmt.Errorf("%s: expected kartui Name(parameters) { markup }: %w", source, ui.ErrTemplate)
	}

	signature := strings.TrimSpace(text[len("kartui "):opening])
	if result.setupSignature != "" {
		signature = result.setupSignature
	}

	declaration, err := parser.ParseFile(
		token.NewFileSet(),
		source,
		"package engine\nfunc "+signature+" {}",
		0,
	)
	if err != nil {
		return result, err
	}

	function, ok := declaration.Decls[0].(*ast.FuncDecl)
	if !ok || function.Type.Results != nil || function.Recv != nil || function.Type.TypeParams != nil ||
		!ast.IsExported(function.Name.Name) {
		return result, fmt.Errorf("%s: component must have an exported name and no result: %w", source, ui.ErrTemplate)
	}

	result.Name = function.Name.Name
	if result.setupSignature != "" && strings.TrimSpace(text[len("kartui "):opening]) != result.Name {
		return result, fmt.Errorf("%s: expected kartui %s without parameters to match setup: %w", source, result.Name, ui.ErrTemplate)
	}

	body := strings.TrimSpace(text[opening+1 : len(text)-1])
	if strings.HasPrefix(body, "setup {") {
		if result.hasSetup {
			return result, fmt.Errorf("%s: duplicate setup block: %w", source, ui.ErrTemplate)
		}

		end, err := expressionEnd(body[len("setup "):])
		if err != nil {
			return result, err
		}

		result.Setup = body[len("setup {") : len("setup ")+end]
		body = body[len("setup ")+end+1:]
		result.Local = true
	}

	for _, field := range function.Type.Params.List {
		for _, name := range field.Names {
			if !ast.IsExported(name.Name) {
				result.Local = true
			}
		}
	}

	if err := result.parameters(source, function); err != nil {
		return result, err
	}

	body, err = result.lowerLoops(body)
	if err != nil {
		return result, fmt.Errorf("%s: %w", source, err)
	}

	body, err = result.lowerConditions(body)
	if err != nil {
		return result, fmt.Errorf("%s: %w", source, err)
	}

	markup, expressions, err := lowerExpressions(body)
	if err != nil {
		return result, fmt.Errorf("%s: %w", source, err)
	}

	markup, layoutStyles, err := expandLayouts(source, markup, layouts)
	if err != nil {
		return result, err
	}

	styleText = strings.Join(append(layoutStyles, styleText), "\n")

	styles, err := parseStyles(source, styleText, theme)
	if err != nil {
		return result, err
	}

	err = result.parseMarkup(markup, expressions)
	if err != nil {
		return result, fmt.Errorf("%s: %w", source, err)
	}

	if err := result.applyStyles(styles); err != nil {
		return result, fmt.Errorf("%s: %w", source, err)
	}

	result.raiseInteractiveHUDSchema()
	result.raiseLayoutSchema(len(layoutStyles) > 0)

	if err := result.Template.ValidateComposition(); err != nil {
		return result, fmt.Errorf(
			"%s: compiled presentation validation (schema=%d modal=%t back=%t elements=%d): %w",
			source, result.Template.Version, result.Template.Modal, result.Template.Back, len(result.Template.Elements), err,
		)
	}

	return result, nil
}

func (component *Component) raiseInteractiveHUDSchema() {
	if component.Template.Modal {
		return
	}

	for _, element := range component.Template.Elements {
		if element.Kind == "button" || element.Kind == "list" {
			component.Template.Version = max(component.Template.Version, ui.SchemaInteractionPolish)

			return
		}
	}
}

func (component *Component) raiseLayoutSchema(usedLayout bool) {
	const layout = ui.StyleBackgroundDisabled | ui.StyleBackgroundImageDisabled |
		ui.StyleColorHover | ui.StyleColorPressed | ui.StyleColorDisabled |
		ui.StyleFlexDirection | ui.StyleAlignItems | ui.StyleMinWidth | ui.StyleMaxWidth | ui.StyleMaxHeight |
		ui.StyleJustifyContent | ui.StyleFlexGrow | ui.StyleMargin | ui.StyleOverflow | ui.StyleTransitionDuration

	if component.Template.Panel.Set2 != 0 || component.Template.ResponsivePanel.Set2 != 0 {
		component.Template.Version = max(component.Template.Version, ui.SchemaInteractionPolish)

		return
	}

	if component.Template.Panel.Set&(ui.StyleTransitionEnter|ui.StyleTransitionExit) != 0 ||
		component.Template.ResponsivePanel.Set&(ui.StyleTransitionEnter|ui.StyleTransitionExit) != 0 {
		component.Template.Version = max(component.Template.Version, ui.SchemaControlTransition)

		return
	}

	for _, element := range component.Template.Elements {
		if element.Style.Set2 != 0 || element.ResponsiveStyle.Set2 != 0 {
			component.Template.Version = max(component.Template.Version, ui.SchemaInteractionPolish)

			return
		}

		if element.Kind == "image" {
			component.Template.Version = max(component.Template.Version, ui.SchemaVisualHierarchy)

			return
		}

		if element.Style.Set&(ui.StyleTextAlign|ui.StyleFontFamily) != 0 ||
			element.ResponsiveStyle.Set&(ui.StyleTextAlign|ui.StyleFontFamily) != 0 {
			component.Template.Version = max(component.Template.Version, ui.SchemaVisualHierarchy)

			return
		}

		if element.Style.Set&ui.StyleTransitionDuration != 0 ||
			element.ResponsiveStyle.Set&ui.StyleTransitionDuration != 0 {
			component.Template.Version = max(component.Template.Version, ui.SchemaControlTransition)

			return
		}
	}

	if usedLayout || component.Template.Panel.Set&(layout|ui.StyleMinHeight) != 0 {
		component.Template.Version = max(component.Template.Version, ui.SchemaLayout)

		return
	}

	for _, element := range component.Template.Elements {
		layoutMinHeight := element.Kind != "button" && element.Kind != "list" && element.Style.Set&ui.StyleMinHeight != 0
		if element.Kind == "panel" || element.Parent != 0 || element.Style.Set&layout != 0 || layoutMinHeight {
			component.Template.Version = max(component.Template.Version, ui.SchemaLayout)

			return
		}
	}
}

// Replace Go expressions with XML-safe tokens. Go's scanner handles braces in
// quoted strings, comments and nested expressions; no regexp parses Go syntax.
func lowerExpressions(body string) (string, map[string]string, error) {
	expressions := map[string]string{}

	var output strings.Builder

	for {
		start := strings.IndexByte(body, '{')
		if start < 0 {
			output.WriteString(body)

			break
		}

		output.WriteString(body[:start])

		fileSet := token.NewFileSet()
		file := fileSet.AddFile("expression", -1, len(body)-start)

		var lexer scanner.Scanner
		lexer.Init(file, []byte(body[start:]), nil, 0)

		depth, end := 0, -1

		for {
			position, kind, _ := lexer.Scan()
			if kind == token.EOF {
				break
			}

			if kind == token.LBRACE {
				depth++
			}

			if kind == token.RBRACE {
				depth--
				if depth == 0 {
					end = file.Offset(position)

					break
				}
			}
		}

		if end < 0 {
			return "", nil, fmt.Errorf("unclosed Go expression: %w", ui.ErrTemplate)
		}

		expression := strings.TrimSpace(body[start+1 : start+end])
		if _, err := parser.ParseExpr(expression); err != nil {
			return "", nil, err
		}

		key := "kartyExpression" + strconv.Itoa(len(expressions))
		if strings.Contains(body, key) {
			return "", nil, fmt.Errorf("reserved expression marker: %w", ui.ErrTemplate)
		}

		expressions[key] = expression
		if strings.HasSuffix(strings.TrimSpace(body[:start]), "=") {
			output.WriteString(strconv.Quote(key))
		} else {
			output.WriteString(key)
		}

		body = body[start+end+1:]
	}

	return output.String(), expressions, nil
}

// XML token handling is explicit so malformed nesting never reaches lowering.
//
//nolint:gocognit // One bounded state machine handles every XML token kind.
func (component *Component) parseMarkup(markup string, expressions map[string]string) error {
	decoder := xml.NewDecoder(strings.NewReader(markup))
	depth := 0
	closed := false
	open := make([]string, 0, 8)

	for {
		node, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		switch node := node.(type) {
		case xml.StartElement:
			if closed || node.Name.Space != "" {
				return ui.ErrTemplate
			}

			if len(open) > 0 && open[len(open)-1] != "panel" && open[len(open)-1] != "_kartyLoop" &&
				open[len(open)-1] != "_kartyIf" {
				return fmt.Errorf("only panels can contain controls: %w", ui.ErrTemplate)
			}

			depth++

			if err := component.startComposition(node, depth, expressions); err != nil {
				return err
			}

			open = append(open, node.Name.Local)
		case xml.EndElement:
			if len(open) == 0 || open[len(open)-1] != node.Name.Local {
				return ui.ErrTemplate
			}

			open = open[:len(open)-1]

			if node.Name.Local == "_kartyLoop" {
				component.activeLoop = ""
				component.loopDepth = 0
			}

			if node.Name.Local == "_kartyIf" {
				component.activeCondition = ""
				component.conditionDepth = 0
			}

			if node.Name.Local == "panel" && depth > 1 {
				if len(component.parents) == 0 {
					return ui.ErrTemplate
				}

				component.parent = component.parents[len(component.parents)-1]
				component.parents = component.parents[:len(component.parents)-1]
			}

			depth--
			if depth == 0 {
				closed = true
			}
		case xml.CharData:
			if err := component.textContent(string(node), expressions); err != nil {
				return err
			}
		case xml.Comment:
		default:
			return ui.ErrTemplate
		}
	}

	if !closed || depth != 0 || len(open) != 0 {
		return ui.ErrTemplate
	}

	return nil
}

func (component *Component) startComposition(node xml.StartElement, depth int, expressions map[string]string) error {
	if node.Name.Local == "_kartyIf" {
		if depth < 2 || component.activeCondition != "" || len(node.Attr) != 1 || node.Attr[0].Name.Local != "index" {
			return ui.ErrTemplate
		}

		index, err := strconv.Atoi(node.Attr[0].Value)
		if err != nil || index < 0 || index >= len(component.conditions) {
			return ui.ErrTemplate
		}

		component.activeCondition = component.conditions[index]
		component.conditionDepth = depth

		return nil
	}

	if node.Name.Local == "_kartyLoop" {
		if depth < 2 || len(node.Attr) != 1 || node.Attr[0].Name.Local != "index" {
			return ui.ErrTemplate
		}

		index, err := strconv.Atoi(node.Attr[0].Value)
		if err != nil || index < 0 || index >= len(component.loops) {
			return ui.ErrTemplate
		}

		component.activeLoop = component.loops[index]
		component.loopDepth = depth

		return nil
	}

	if component.activeLoop != "" {
		if depth != component.loopDepth+1 || !ast.IsExported(node.Name.Local) {
			return fmt.Errorf("loops must contain child components: %w", ui.ErrTemplate)
		}

		if err := component.addChild(node, expressions); err != nil {
			return err
		}

		return nil
	}

	return component.startElement(node, depth, expressions)
}

func (component *Component) addElement(node xml.StartElement, expressions map[string]string) error {
	if ast.IsExported(node.Name.Local) {
		return component.addChild(node, expressions)
	}

	identifier := uint32(len(component.Bindings) + 1)
	element := ui.Element{
		ID: identifier, Parent: component.parent,
		Name: "element_" + strconv.Itoa(int(identifier)), Kind: node.Name.Local,
	}
	binding := Binding{ID: identifier, Enabled: "true"}
	binding.Visible = component.activeCondition

	seen := map[string]bool{}
	class := ""

	for _, attr := range node.Attr {
		if attr.Name.Space != "" || seen[attr.Name.Local] {
			return ui.ErrTemplate
		}

		seen[attr.Name.Local] = true

		if attr.Name.Local == "class" {
			if !validCSSName(attr.Value) {
				return ui.ErrTemplate
			}

			class = attr.Value

			continue
		}

		expression, ok := expressions[attr.Value]
		if !ok {
			return fmt.Errorf("%s requires a Go expression in braces: %w", attr.Name.Local, ui.ErrTemplate)
		}

		switch attr.Name.Local {
		case "enabled":
			binding.Enabled = expression
		case "onClick":
			binding.Click = expression
			element.Action = identifier
		case "rows":
			binding.Rows = expression
		default:
			return fmt.Errorf("unsupported %s attribute %s: %w", element.Kind, attr.Name.Local, ui.ErrTemplate)
		}
	}

	if (element.Kind == "list") != (binding.Rows != "") ||
		(element.Kind == "label" && binding.Click != "") ||
		(element.Kind == "panel" && (binding.Click != "" || binding.Rows != "" || binding.Enabled != "true")) {
		return ui.ErrTemplate
	}

	component.Template.Elements = append(component.Template.Elements, element)
	component.Bindings = append(component.Bindings, binding)
	component.elementClasses = append(component.elementClasses, class)

	return nil
}

func (component *Component) startElement(node xml.StartElement, depth int, expressions map[string]string) error {
	if depth > 1 {
		if err := component.addElement(node, expressions); err != nil {
			return err
		}

		if node.Name.Local == "panel" {
			component.parents = append(component.parents, component.parent)
			component.parent = component.Template.Elements[len(component.Template.Elements)-1].ID
		}

		return nil
	}

	if node.Name.Local != "panel" {
		return fmt.Errorf("expected one root panel: %w", ui.ErrTemplate)
	}

	if len(node.Attr) > maxPanelAttributes {
		return fmt.Errorf("panel supports modal, class, and onBack: %w", ui.ErrTemplate)
	}

	seen := map[string]bool{}
	for _, attr := range node.Attr {
		if attr.Name.Space != "" || seen[attr.Name.Local] {
			return ui.ErrTemplate
		}

		seen[attr.Name.Local] = true
		if attr.Name.Local == "class" && validCSSName(attr.Value) {
			component.panelClass = attr.Value

			continue
		}

		if attr.Name.Local == "onBack" {
			expression, ok := expressions[attr.Value]
			if !ok {
				return fmt.Errorf("onBack requires a Go expression in braces: %w", ui.ErrTemplate)
			}

			component.Back = expression
			component.Template.Back = true
			component.Template.Version = max(component.Template.Version, ui.SchemaInteractionPolish)

			continue
		}

		if attr.Name.Local != "modal" || (attr.Value != "true" && attr.Value != "false") {
			return fmt.Errorf("unsupported panel attribute %s: %w", attr.Name.Local, ui.ErrTemplate)
		}

		component.Template.Modal = attr.Value == "true"
	}

	return nil
}

func (component *Component) applyStyles(rules styleRules) error {
	if err := component.applyBaseStyles(rules.base, rules.responsive); err != nil {
		return err
	}

	if err := component.applyResponsiveStyles(rules.responsiveMaxWidth, rules.responsive); err != nil {
		return err
	}

	component.raiseStyleSchema(len(rules.base) > 0 || len(rules.responsive) > 0)

	return nil
}

func (component *Component) applyBaseStyles(base, responsive map[string]ui.Style) error {
	used := map[string]bool{}

	if style, exists := base["panel"]; exists {
		component.Template.Panel = mergeStyle(component.Template.Panel, style)
		used["panel"] = true
	}

	if component.panelClass != "" {
		selector := "." + component.panelClass

		style, exists := base[selector]
		_, responsiveExists := responsive[selector]

		if !exists && !responsiveExists {
			return fmt.Errorf("class %s has no style rule: %w", component.panelClass, ui.ErrTemplate)
		}

		if exists {
			component.Template.Panel = mergeStyle(component.Template.Panel, style)
			used[selector] = true
		}
	}

	for index := range component.Template.Elements {
		element := &component.Template.Elements[index]
		if style, exists := base[element.Kind]; exists {
			element.Style = mergeStyle(element.Style, style)
			used[element.Kind] = true
		}

		if index < len(component.elementClasses) && component.elementClasses[index] != "" {
			selector := "." + component.elementClasses[index]

			style, exists := base[selector]
			_, responsiveExists := responsive[selector]

			if !exists && !responsiveExists {
				return fmt.Errorf("class %s has no style rule: %w", component.elementClasses[index], ui.ErrTemplate)
			}

			if exists {
				element.Style = mergeStyle(element.Style, style)
				used[selector] = true
			}
		}
	}

	for selector := range base {
		if !used[selector] && strings.HasPrefix(selector, ".") {
			return fmt.Errorf("unused style selector %s: %w", selector, ui.ErrTemplate)
		}
	}

	return nil
}

func (component *Component) applyResponsiveStyles(maxWidth uint16, rules map[string]ui.Style) error {
	if maxWidth == 0 {
		return nil
	}

	component.Template.ResponsiveMaxWidth = maxWidth
	used := map[string]bool{}
	responsivePanel := component.Template.Panel
	panelChanged := false

	if style, exists := rules["panel"]; exists {
		responsivePanel = mergeStyle(responsivePanel, style)
		used["panel"], panelChanged = true, true
	}

	if component.panelClass != "" {
		selector := "." + component.panelClass
		if style, exists := rules[selector]; exists {
			responsivePanel = mergeStyle(responsivePanel, style)
			used[selector], panelChanged = true, true
		}
	}

	if panelChanged {
		component.Template.ResponsivePanel = responsivePanel
	}

	for index := range component.Template.Elements {
		component.applyResponsiveElement(index, rules, used)
	}

	for selector := range rules {
		if !used[selector] && strings.HasPrefix(selector, ".") {
			return fmt.Errorf("unused responsive style selector %s: %w", selector, ui.ErrTemplate)
		}
	}

	component.Template.Version = max(component.Template.Version, ui.SchemaResponsive)

	return nil
}

func (component *Component) applyResponsiveElement(index int, rules map[string]ui.Style, used map[string]bool) {
	element := &component.Template.Elements[index]
	responsiveStyle := element.Style
	changed := false

	if style, exists := rules[element.Kind]; exists {
		responsiveStyle = mergeStyle(responsiveStyle, style)
		used[element.Kind], changed = true, true
	}

	if index < len(component.elementClasses) && component.elementClasses[index] != "" {
		selector := "." + component.elementClasses[index]
		if style, exists := rules[selector]; exists {
			responsiveStyle = mergeStyle(responsiveStyle, style)
			used[selector], changed = true, true
		}
	}

	if changed {
		element.ResponsiveStyle = responsiveStyle
	}
}

func (component *Component) raiseStyleSchema(styled bool) {
	if !styled {
		return
	}

	component.Template.Version = max(component.Template.Version, ui.SchemaStyle)

	const images = ui.StyleBackgroundImage | ui.StyleBackgroundImageHover | ui.StyleBackgroundImagePressed |
		ui.StyleContentImage
	if component.Template.Panel.Set&images != 0 || component.Template.ResponsivePanel.Set&images != 0 {
		component.Template.Version = max(component.Template.Version, ui.SchemaImageStyle)
	}

	for _, element := range component.Template.Elements {
		if element.Style.Set&images != 0 || element.ResponsiveStyle.Set&images != 0 {
			component.Template.Version = max(component.Template.Version, ui.SchemaImageStyle)

			return
		}
	}
}

func (component *Component) textContent(value string, expressions map[string]string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	index := len(component.Bindings) - 1
	if index < 0 || component.Bindings[index].Child != "" {
		return ui.ErrTemplate
	}

	if kind := component.Template.Elements[index].Kind; kind != "label" && kind != "button" {
		return ui.ErrTemplate
	}

	if component.Bindings[index].Text != "" || component.Bindings[index].DefaultText != "" {
		return fmt.Errorf("use one text expression or literal per control: %w", ui.ErrTemplate)
	}

	if expression, ok := expressions[value]; ok {
		component.Bindings[index].Text = expression

		return nil
	}

	if strings.Contains(value, "kartyExpression") {
		return fmt.Errorf("combine text inside one Go expression: %w", ui.ErrTemplate)
	}

	component.Template.Elements[index].Text = value
	component.Bindings[index].DefaultText = value

	return nil
}

// DecodeSource preserves legacy JSON assets while accepting the new authoring
// format. Only the canonical bounded asset is embedded; markup never ships loose.
func DecodeSource(source string, data []byte) (ui.Template, error) {
	return DecodeSourceWithTheme(source, data, DefaultTheme())
}

func DecodeSourceWithTheme(source string, data []byte, theme Theme) (ui.Template, error) {
	if !strings.HasSuffix(source, ".ui") && !strings.HasSuffix(source, ".kui") {
		return ui.Decode(data)
	}

	component, err := CompileWithTheme(source, data, theme)
	if err != nil {
		return ui.Template{}, err
	}

	if component.Local {
		return ui.Template{}, fmt.Errorf("%s: level UI cannot contain client setup code: %w", source, ui.ErrTemplate)
	}

	if component.Back != "" {
		return ui.Template{}, fmt.Errorf(
			"%s: dynamic level UI back action requires a client component: %w",
			source,
			ui.ErrTemplate,
		)
	}

	for _, binding := range component.Bindings {
		if binding.Text != "" || binding.Rows != "" || binding.Click != "" || binding.Enabled != "true" {
			return ui.Template{}, fmt.Errorf(
				"%s: dynamic level UI bindings require a client component; only static level templates are supported yet: %w",
				source,
				ui.ErrTemplate,
			)
		}
	}

	return component.Template, nil
}

func (component *Component) parameters(source string, function *ast.FuncDecl) error {
	seen := map[string]bool{}

	for _, field := range function.Type.Params.List {
		var output bytes.Buffer
		if err := format.Node(&output, token.NewFileSet(), field.Type); err != nil {
			return err
		}

		kind := output.String()
		if !component.Local {
			switch kind {
			case "string", "bool", "[]UIRow", "func()", "func(uint32)":
			default:
				return fmt.Errorf("%s: unsupported presentation parameter type %s: %w", source, kind, ui.ErrTemplate)
			}
		}

		if len(field.Names) == 0 {
			return fmt.Errorf("%s: parameters must be named: %w", source, ui.ErrTemplate)
		}

		for _, name := range field.Names {
			if (!component.Local && !ast.IsExported(name.Name)) || seen[name.Name] || name.Name == "invalidate" ||
				strings.HasPrefix(name.Name, "_karty") {
				return fmt.Errorf("%s: parameter %s must be unique and exported: %w", source, name, ui.ErrTemplate)
			}

			seen[name.Name] = true
			component.Parameters = append(component.Parameters, Parameter{Name: name.Name, Type: kind})
		}
	}

	return nil
}

func (component *Component) preamble(source, text string) (string, error) {
	if strings.HasPrefix(text, "kartui ") {
		return text, nil
	}

	index := strings.Index(text, "\nkartui ")
	if index < 0 {
		return "", fmt.Errorf("%s: imports and types must precede a kartui declaration: %w", source, ui.ErrTemplate)
	}

	var err error

	component.Preamble, err = component.separateSetup(text[:index])
	if err != nil {
		return "", err
	}

	parsed, err := parser.ParseFile(token.NewFileSet(), source, "package main\n"+component.Preamble, 0)
	if err != nil {
		return "", err
	}

	for _, declaration := range parsed.Decls {
		decl, ok := declaration.(*ast.GenDecl)
		if !ok || (decl.Tok != token.IMPORT && decl.Tok != token.TYPE) {
			return "", ui.ErrTemplate
		}
	}

	text = strings.TrimSpace(text[index:])
	component.Local = component.Local || len(parsed.Decls) > 0

	return text, nil
}
