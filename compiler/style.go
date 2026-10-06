package uicompiler

import (
	"fmt"
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/karty-game/karty-ui/schema"
	"github.com/pelletier/go-toml/v2"
)

// Theme contains build-time tokens and resolved asset references. The host
// never parses authoring TOML or performs theme-token lookups.
type Theme struct {
	Values map[string]string
	Images map[string]ui.Image
}

const rgbHexLength = 6

func DefaultTheme() Theme {
	return Theme{Values: map[string]string{
		"colors.surface":          "#131a22f0",
		"colors.primary":          "#2d4458ff",
		"colors.primary-hover":    "#416280ff",
		"colors.primary-pressed":  "#1e3044ff",
		"colors.accent":           "#70d6ffff",
		"colors.text":             "#ffffffff",
		"spacing.panel":           "16",
		"spacing.gap":             "8",
		"typography.body":         "20",
		"typography.title":        "28",
		"typography.body-family":  "body",
		"typography.title-family": "display",
		"typography.mono-family":  "mono",
		"controls.height":         "44",
	}, Images: map[string]ui.Image{}}
}

// ParseTheme accepts a bounded theme.toml with a built-in base and typed groups.
func ParseTheme(source string, data []byte) (Theme, error) {
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return Theme{}, fmt.Errorf("%s: %w", source, err)
	}

	extends, ok := document["extends"].(string)
	if !ok || extends != "dark" {
		return Theme{}, fmt.Errorf("%s: supported base theme is dark: %w", source, ui.ErrTemplate)
	}

	delete(document, "extends")

	theme := DefaultTheme()
	allowed := map[string]bool{"colors": true, "spacing": true, "typography": true, "controls": true, "images": true}

	for group, raw := range document {
		values, ok := raw.(map[string]any)
		if !ok || !allowed[group] {
			return Theme{}, fmt.Errorf("%s: invalid theme group %s: %w", source, group, ui.ErrTemplate)
		}

		if group == "images" {
			for name, value := range values {
				image, err := parseThemeImage(name, value)
				if err != nil {
					return Theme{}, fmt.Errorf("%s: image %s: %w", source, name, err)
				}

				theme.Images[name] = image
			}

			continue
		}

		for name, rawValue := range values {
			if !validCSSName(name) {
				return Theme{}, ui.ErrTemplate
			}

			var value string

			switch typed := rawValue.(type) {
			case string:
				value = typed
			case int64:
				value = strconv.FormatInt(typed, 10)
			default:
				return Theme{}, fmt.Errorf("%s: invalid theme token %s.%s: %w", source, group, name, ui.ErrTemplate)
			}

			theme.Values[group+"."+name] = value
		}
	}

	return theme, nil
}

func parseThemeImage(name string, raw any) (ui.Image, error) {
	values, isTable := raw.(map[string]any)
	if !isTable || !validCSSName(name) || len(values) != 2 {
		return ui.Image{}, ui.ErrTemplate
	}

	asset, assetValid := values["asset"].(string)
	if !assetValid || asset == "" || len(asset) > ui.MaxNameBytes {
		return ui.Image{}, ui.ErrTemplate
	}

	rawSlice, ok := values["slice"].([]any)
	if !ok || len(rawSlice) != 4 {
		return ui.Image{}, ui.ErrTemplate
	}

	parts := [4]uint16{}

	for index, rawPart := range rawSlice {
		part, ok := rawPart.(int64)
		if !ok || part < 0 || part > 256 {
			return ui.Image{}, ui.ErrTemplate
		}

		parts[index] = uint16(part)
	}

	return ui.Image{
		Scope: ui.ImageScopeGame, Name: asset,
		Top: parts[0], Right: parts[1], Bottom: parts[2], Left: parts[3],
	}, nil
}

// ImageNames returns the statically referenced project textures in a theme.
func (theme Theme) ImageNames() []string {
	names := make([]string, 0, len(theme.Images))

	seen := map[string]bool{}
	for _, image := range theme.Images {
		if image.Scope == ui.ImageScopeGame && !seen[image.Name] {
			seen[image.Name] = true
			names = append(names, image.Name)
		}
	}

	return names
}

// ResolveLevelImages binds a level-local theme to its stable texture IDs.
func (theme Theme) ResolveLevelImages(ids map[string]uint32, dimensions map[string][2]int) (Theme, error) {
	resolved := Theme{Values: theme.Values, Images: make(map[string]ui.Image, len(theme.Images))}
	for token, image := range theme.Images {
		assetID, exists := ids[image.Name]
		if !exists {
			return Theme{}, fmt.Errorf("theme image %s references undeclared level texture %q: %w", token, image.Name, ui.ErrTemplate)
		}

		if err := validateSlice(image, dimensions[image.Name]); err != nil {
			return Theme{}, fmt.Errorf("theme image %s: %w", token, err)
		}

		image.Scope, image.AssetID, image.Name = ui.ImageScopeLevel, assetID, ""
		resolved.Images[token] = image
	}

	return resolved, nil
}

// ValidateGameImages checks project declarations and nine-slice geometry.
func (theme Theme) ValidateGameImages(dimensions map[string][2]int) error {
	for token, image := range theme.Images {
		dimension, exists := dimensions[image.Name]
		if !exists {
			return fmt.Errorf("theme image %s references undeclared project texture %q: %w", token, image.Name, ui.ErrTemplate)
		}

		if err := validateSlice(image, dimension); err != nil {
			return fmt.Errorf("theme image %s: %w", token, err)
		}
	}

	return nil
}

func validateSlice(image ui.Image, dimension [2]int) error {
	if int(image.Left)+int(image.Right) >= dimension[0] || int(image.Top)+int(image.Bottom) >= dimension[1] {
		return fmt.Errorf("slice leaves no stretchable center in %dx%d texture: %w", dimension[0], dimension[1], ui.ErrTemplate)
	}

	return nil
}

func splitStyle(text string) (string, string, error) {
	marker := strings.LastIndex(text, "\nstyle {")
	if marker < 0 {
		return text, "", nil
	}

	body := strings.TrimSpace(text[marker+1:])
	if !strings.HasPrefix(body, "style {") || !strings.HasSuffix(body, "}") {
		return "", "", ui.ErrTemplate
	}

	return strings.TrimSpace(text[:marker]), body[len("style {") : len(body)-1], nil
}

type styleRules struct {
	base               map[string][]styleDeclaration
	responsive         map[string][]styleDeclaration
	responsiveMaxWidth uint16
	warnings           []Diagnostic
	locations          map[string]SourceLocation
}

func parseStyleBlocks(document styleSource, start, end int, theme Theme, rules *styleRules, responsive bool) error {
	if len(document.lineStarts) == 0 {
		document.lineStarts = sourceLineStarts(document.original)
	}

	for start < end {
		text := document.text[start:end]

		start += len(text) - len(strings.TrimLeft(text, " \t\r\n"))
		if start == end {
			break
		}

		text = document.text[start:end]

		open := strings.IndexByte(text, '{')
		if open < 1 {
			return sourceError(document.position(start), "expected selector { declarations }", ui.ErrTemplate)
		}

		closeAt, err := styleBlockEnd(text, open)
		if err != nil {
			return sourceError(document.position(start), "unclosed style block", err)
		}

		header := strings.TrimSpace(text[:open])
		if err := parseStyleBlock(document, header, start, start+open+1, start+closeAt, theme, rules, responsive); err != nil {
			return err
		}

		start += closeAt + 1
	}

	return nil
}

func parseStyleBlock(
	document styleSource,
	header string,
	headerStart, bodyStart, bodyEnd int,
	theme Theme,
	rules *styleRules,
	responsive bool,
) error {
	if strings.HasPrefix(header, "@media") {
		return parseResponsiveBlock(document, header, headerStart, bodyStart, bodyEnd, theme, rules, responsive)
	}

	target := rules.base
	if responsive {
		target = rules.responsive
	}

	return parseStyleRule(document, header, headerStart, bodyStart, bodyEnd, theme, target, rules)
}

func parseResponsiveBlock(
	document styleSource,
	header string,
	headerStart, bodyStart, bodyEnd int,
	theme Theme,
	rules *styleRules,
	nested bool,
) error {
	location := document.position(headerStart)
	if nested {
		appendStyleWarning(
			&rules.warnings,
			Diagnostic{SourceLocation: location, Message: "nested media query ignored: unsupported nesting"},
		)

		return nil
	}

	maxWidth, err := parseMediaMaxWidth(header)
	if err != nil {
		appendStyleWarning(
			&rules.warnings,
			Diagnostic{SourceLocation: location, Message: fmt.Sprintf("media query %q ignored: %v", header, err)},
		)

		return nil
	}

	if rules.responsiveMaxWidth != 0 && rules.responsiveMaxWidth != maxWidth {
		appendStyleWarning(
			&rules.warnings,
			Diagnostic{
				SourceLocation: location,
				Message:        fmt.Sprintf("media query %q ignored: use one responsive max-width per component", header),
			},
		)

		return nil
	}

	rules.responsiveMaxWidth = maxWidth

	return parseStyleBlocks(document, bodyStart, bodyEnd, theme, rules, true)
}

func styleBlockEnd(text string, open int) (int, error) {
	depth := 0

	for index := open; index < len(text); index++ {
		switch text[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index, nil
			}
		}
	}

	return 0, ui.ErrTemplate
}

func parseMediaMaxWidth(header string) (uint16, error) {
	condition := strings.TrimSpace(strings.TrimPrefix(header, "@media"))
	if !strings.HasPrefix(condition, "(") || !strings.HasSuffix(condition, ")") {
		return 0, fmt.Errorf("expected @media (max-width: N): %w", ui.ErrTemplate)
	}

	name, value, ok := strings.Cut(strings.TrimSpace(condition[1:len(condition)-1]), ":")
	parsed, err := strconv.ParseUint(strings.TrimSpace(value), 10, 16)

	if !ok || strings.TrimSpace(name) != "max-width" || err != nil || parsed < ui.MinResponsiveWidth || parsed > ui.MaxResponsiveWidth {
		return 0, fmt.Errorf("responsive max-width must be %d through %d: %w", ui.MinResponsiveWidth, ui.MaxResponsiveWidth, ui.ErrTemplate)
	}

	return uint16(parsed), nil
}

func parseStyleRule(
	document styleSource,
	selector string,
	selectorStart, bodyStart, bodyEnd int,
	theme Theme,
	target map[string][]styleDeclaration,
	rules *styleRules,
) error {
	baseSelector, state, valid := splitSelector(selector)
	if !valid {
		appendStyleWarning(
			&rules.warnings,
			Diagnostic{
				SourceLocation: document.position(selectorStart),
				Message:        fmt.Sprintf("style selector %q ignored: unsupported selector", selector),
			},
		)

		return nil
	}

	key, valid := scopedSelectorKey(document.scope, baseSelector)
	if !valid {
		appendStyleWarning(
			&rules.warnings,
			Diagnostic{
				SourceLocation: document.position(selectorStart),
				Message:        "qualified layout overrides belong in the consuming component",
			},
		)

		return nil
	}

	if _, exists := target[key]; !exists {
		target[key] = nil
	}

	if rules.locations == nil {
		rules.locations = map[string]SourceLocation{}
	}

	rules.locations[key] = document.position(selectorStart)
	seen := map[string]bool{}

	offset := bodyStart
	for part := range strings.SplitSeq(document.text[bodyStart:bodyEnd], ";") {
		propertyOffset := offset + len(part) - len(strings.TrimLeft(part, " \t\r\n"))
		offset += len(part) + 1

		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}

		property, value, hasValue := strings.Cut(raw, ":")
		property, value = canonicalStyleProperty(strings.TrimSpace(property)), strings.TrimSpace(value)

		declaration := styleDeclaration{
			source:   document.source,
			selector: selector,
			property: property,
			value:    value,
			location: document.position(propertyOffset),
		}
		if !hasValue || !validCSSName(property) {
			declaration.warn(&rules.warnings, "expected property: value")

			continue
		}

		if seen[property] {
			declaration.warn(&rules.warnings, "duplicate property")

			continue
		}

		resolved, err := stateProperty(property, state)
		if err != nil {
			declaration.warn(&rules.warnings, err.Error())

			continue
		}

		if err := applyStyleProperty(&declaration.style, resolved, value, theme); err != nil {
			declaration.warn(&rules.warnings, err.Error())

			continue
		}

		if !stylePropertyValid(declaration.style, "root") && !stylePropertyValid(declaration.style, "panel") &&
			!stylePropertyValid(
				declaration.style,
				"button",
			) && !stylePropertyValid(declaration.style, "image") && !stylePropertyValid(declaration.style, "label") {
			declaration.warn(&rules.warnings, "value is outside the supported range")

			continue
		}

		seen[property] = true

		target[key] = append(target[key], declaration)
	}

	return nil
}

func applyStyleProperty(style *ui.Style, property, value string, theme Theme) error {
	property = canonicalStyleProperty(property)
	if !supportedStyleProperty(property) {
		return fmt.Errorf("unknown style property %s: %w", property, ui.ErrTemplate)
	}

	if strings.HasPrefix(property, "background-image") || property == "image" || property == "icon" {
		token, valid := strings.CutPrefix(value, "theme.images.")

		image, exists := theme.Images[token]
		if !valid || !exists {
			return fmt.Errorf("unknown theme image %s: %w", value, ui.ErrTemplate)
		}

		return setStyleImage(style, property, image)
	}

	if token, themed := strings.CutPrefix(value, "theme."); themed {
		var exists bool

		value, exists = theme.Values[token]
		if !exists {
			return fmt.Errorf("unknown theme token %s: %w", token, ui.ErrTemplate)
		}
	}

	err := setStyle(style, property, value)
	if err != nil && (property == "width" || property == "height") {
		return fmt.Errorf("expected auto, 0..2048 logical pixels or 0..100%% with up to two decimal places: %w", err)
	}

	return err
}

// supportedStyleProperty reports the complete public CSS-like declaration
// surface. Keep it in sync with docs tests: undocumented authoring is not shipped.
func supportedStyleProperty(property string) bool {
	return slices.Contains(supportedStylePropertyNames(), property)
}

func supportedStylePropertyNames() []string {
	return []string{
		"background", "background-hover", "background-focus", "background-pressed", "background-disabled",
		"color", "color-hover", "color-focus", "color-pressed", "color-disabled",
		"background-image", "background-image-hover", "background-image-focus", "background-image-pressed", "background-image-disabled",
		"padding", "gap", "flex-direction", "align-items", "justify-content", "overflow",
		"position", "left", "right", "top", "bottom",
		"font-size", "font-family", "text-align",
		"image", "image-fit", "icon", "icon-position", "icon-size", "icon-gap", "tint",
		"min-width", "min-height", "max-width", "max-height", "flex-grow", "margin",
		"width", "height",
		"transition-duration", "transition-delay", "transition-easing", "transition-enter", "transition-exit",
	}
}

func setStyleImage(style *ui.Style, property string, image ui.Image) error {
	switch property {
	case "background-image":
		style.Set |= ui.StyleBackgroundImage
		style.BackgroundImage = image
	case "background-image-hover":
		style.Set |= ui.StyleBackgroundImageHover
		style.BackgroundImageHover = image
	case "background-image-focus":
		style.Set2 |= ui.Style2BackgroundImageFocus
		style.BackgroundImageFocus = image
	case "background-image-pressed":
		style.Set |= ui.StyleBackgroundImagePressed
		style.BackgroundImagePressed = image
	case "background-image-disabled":
		style.Set |= ui.StyleBackgroundImageDisabled
		style.BackgroundImageDisabled = image
	case "image":
		style.Set |= ui.StyleContentImage
		style.ContentImage = image
	case "icon":
		style.Set2 |= ui.Style2Icon
		style.Icon = image
	default:
		return fmt.Errorf("unknown style property %s: %w", property, ui.ErrTemplate)
	}

	return nil
}

func validCSSName(value string) bool {
	if value == "" {
		return false
	}

	for index, character := range value {
		if !unicode.IsLower(character) && character != '-' && (index <= 0 || !unicode.IsDigit(character)) {
			return false
		}
	}

	return true
}

func validSelector(value string) bool {
	if scope, class, qualified := strings.Cut(value, "."); qualified && scope != "" {
		return token.IsIdentifier(scope) && ast.IsExported(scope) && validCSSName(class)
	}

	if strings.HasPrefix(value, ".") {
		return validCSSName(value[1:])
	}

	switch value {
	case "panel", "label", "button", "list", "image", "checkbox", "combo", "input", "slider", "tabs", "tab":
		return true
	}

	return false
}

func scopedSelectorKey(scope, selector string) (string, bool) {
	if owner, class, qualified := strings.Cut(selector, "."); qualified && owner != "" {
		if scope != "" {
			return "", false
		}

		return owner + "/." + class, true
	}

	if scope != "" {
		return scope + "/" + selector, true
	}

	return selector, true
}

func displayStyleSelector(key string) string {
	scope, selector, qualified := strings.Cut(key, "/")
	if !qualified {
		return key
	}

	if strings.HasPrefix(selector, ".") {
		return scope + selector
	}

	return scope + " " + selector
}

func splitSelector(value string) (string, string, bool) {
	base, state, hasState := strings.Cut(value, ":")
	if !validSelector(base) || strings.Contains(state, ":") {
		return "", "", false
	}

	if !hasState {
		return base, "", true
	}

	if state != "hover" && state != "focus" && state != "pressed" && state != "disabled" {
		return "", "", false
	}

	return base, state, true
}

func stateProperty(property, state string) (string, error) {
	if state == "" {
		return property, nil
	}

	if property != "background" && property != "background-image" && property != "color" {
		return "", fmt.Errorf("property %s has no interactive state: %w", property, ui.ErrTemplate)
	}

	return property + "-" + state, nil
}

//nolint:gocognit,gocyclo,maintidx // Typed property decoding is intentionally centralized and exhaustive.
func setStyle(style *ui.Style, property, value string) error {
	if property == "width" || property == "height" {
		length, err := parseStyleLength(value)
		if err != nil {
			return err
		}

		if property == "width" {
			style.Set2 |= ui.Style2Width
			style.Width = length
		} else {
			style.Set2 |= ui.Style2Height
			style.Height = length
		}

		return nil
	}

	const (
		maxFlexGrow = 16
		maxMargin   = 256
	)

	colors := map[string]struct {
		flag   uint32
		target *uint32
	}{
		"background":          {ui.StyleBackground, &style.Background},
		"background-hover":    {ui.StyleBackgroundHover, &style.BackgroundHover},
		"background-pressed":  {ui.StyleBackgroundPressed, &style.BackgroundPressed},
		"color":               {ui.StyleColor, &style.Color},
		"background-disabled": {ui.StyleBackgroundDisabled, &style.BackgroundDisabled},
		"background-focus":    {ui.Style2BackgroundFocus, &style.BackgroundFocus},
		"color-hover":         {ui.StyleColorHover, &style.ColorHover},
		"color-pressed":       {ui.StyleColorPressed, &style.ColorPressed},
		"color-disabled":      {ui.StyleColorDisabled, &style.ColorDisabled},
		"color-focus":         {ui.Style2ColorFocus, &style.ColorFocus},
	}
	if field, ok := colors[property]; ok {
		parsed, err := parseColor(value)
		if err != nil {
			return err
		}

		if property == "background-focus" || property == "color-focus" {
			style.Set2 |= field.flag
		} else {
			style.Set |= field.flag
		}

		*field.target = parsed

		return nil
	}

	if property == "flex-direction" {
		direction, valid := map[string]uint8{"column": ui.FlexDirectionColumn, "row": ui.FlexDirectionRow}[value]
		if !valid {
			return fmt.Errorf("invalid flex-direction value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleFlexDirection
		style.FlexDirection = direction

		return nil
	}

	if property == "align-items" {
		alignment, valid := map[string]uint8{
			"start": ui.AlignItemsStart, "center": ui.AlignItemsCenter,
			"end": ui.AlignItemsEnd, "stretch": ui.AlignItemsStretch,
		}[value]
		if !valid {
			return fmt.Errorf("invalid align-items value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleAlignItems
		style.AlignItems = alignment

		return nil
	}

	if property == "justify-content" {
		justification, valid := map[string]uint8{
			"start": ui.JustifyContentStart, "center": ui.JustifyContentCenter,
			"end": ui.JustifyContentEnd, "space-between": ui.JustifyContentSpaceBetween,
		}[value]
		if !valid {
			return fmt.Errorf("invalid justify-content value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleJustifyContent
		style.JustifyContent = justification

		return nil
	}

	if property == "text-align" {
		alignment, valid := map[string]uint8{
			"left": ui.TextAlignStart, "center": ui.TextAlignCenter, "right": ui.TextAlignEnd,
		}[value]
		if !valid {
			return fmt.Errorf("invalid text-align value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleTextAlign
		style.TextAlign = alignment

		return nil
	}

	if property == "font-family" {
		family, valid := map[string]uint8{
			"body": ui.FontFamilyBody, "display": ui.FontFamilyDisplay, "mono": ui.FontFamilyMono,
		}[value]
		if !valid {
			return fmt.Errorf("invalid font-family value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleFontFamily
		style.FontFamily = family

		return nil
	}

	if property == "overflow" {
		overflow, valid := map[string]uint8{
			"visible": ui.OverflowVisible, "scroll": ui.OverflowScroll,
		}[value]
		if !valid {
			return fmt.Errorf("invalid overflow value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleOverflow
		style.Overflow = overflow

		return nil
	}

	if property == "transition-easing" {
		easing, valid := map[string]uint8{
			"linear": ui.TransitionEasingLinear, "ease-out": ui.TransitionEasingOut,
			"ease-in-out": ui.TransitionEasingInOut,
		}[value]
		if !valid {
			return fmt.Errorf("invalid transition-easing value %q: %w", value, ui.ErrTemplate)
		}

		style.Set2 |= ui.Style2TransitionEasing
		style.TransitionEasing = easing

		return nil
	}

	if property == "position" {
		if value != "absolute" {
			return fmt.Errorf("invalid position value %q: %w", value, ui.ErrTemplate)
		}

		style.Set2 |= ui.Style2Position
		style.Position = ui.PositionOverlay

		return nil
	}

	if property == "image-fit" {
		fit, valid := map[string]uint8{
			"stretch": ui.ImageFitStretch, "contain": ui.ImageFitContain, "cover": ui.ImageFitCover,
		}[value]
		if !valid {
			return fmt.Errorf("invalid image-fit value %q: %w", value, ui.ErrTemplate)
		}

		style.Set2 |= ui.Style2ImageFit
		style.ImageFit = fit

		return nil
	}

	if property == "icon-position" {
		position, valid := map[string]uint8{"start": ui.IconPositionStart, "end": ui.IconPositionEnd}[value]
		if !valid {
			return fmt.Errorf("invalid icon-position value %q: %w", value, ui.ErrTemplate)
		}

		style.Set2 |= ui.Style2IconPosition
		style.IconPosition = position

		return nil
	}

	if property == "tint" {
		parsed, err := parseColor(value)
		if err != nil {
			return err
		}

		style.Set2 |= ui.Style2Tint
		style.Tint = parsed

		return nil
	}

	if property == "transition-enter" || property == "transition-exit" {
		prefix := "slide-from-"
		if property == "transition-exit" {
			prefix = "slide-to-"
		}

		direction, valid := map[string]uint8{
			prefix + "left": ui.TransitionLeft, prefix + "right": ui.TransitionRight,
			prefix + "top": ui.TransitionTop, prefix + "bottom": ui.TransitionBottom,
		}[value]
		if !valid {
			return fmt.Errorf("invalid %s value %q: %w", property, value, ui.ErrTemplate)
		}

		if property == "transition-enter" {
			style.Set |= ui.StyleTransitionEnter
			style.TransitionEnter = direction
		} else {
			style.Set |= ui.StyleTransitionExit
			style.TransitionExit = direction
		}

		return nil
	}

	if property != "flex-grow" && !strings.HasPrefix(property, "transition-") {
		value = strings.TrimSuffix(value, "px")
	}

	parsed, err := strconv.ParseUint(value, 10, 16)
	if err != nil || parsed > 2048 {
		return fmt.Errorf("invalid %s value %q: %w", property, value, ui.ErrTemplate)
	}

	switch property {
	case "padding":
		style.Set |= ui.StylePadding
		style.Padding = uint16(parsed)
	case "gap":
		style.Set |= ui.StyleGap
		style.Gap = uint16(parsed)
	case "font-size":
		style.Set |= ui.StyleFontSize
		style.FontSize = uint16(parsed)
	case "min-height":
		style.Set |= ui.StyleMinHeight
		style.MinHeight = uint16(parsed)
	case "min-width":
		style.Set |= ui.StyleMinWidth
		style.MinWidth = uint16(parsed)
	case "max-width":
		style.Set |= ui.StyleMaxWidth
		style.MaxWidth = uint16(parsed)
	case "max-height":
		style.Set |= ui.StyleMaxHeight
		style.MaxHeight = uint16(parsed)
	case "flex-grow":
		if parsed > maxFlexGrow {
			return fmt.Errorf("invalid flex-grow value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleFlexGrow
		style.FlexGrow = uint8(parsed)
	case "margin":
		if parsed > maxMargin {
			return fmt.Errorf("invalid margin value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleMargin
		style.Margin = uint16(parsed)
	case "transition-duration":
		if parsed > ui.MaxTransitionMilliseconds {
			return fmt.Errorf("invalid transition-duration value %q: %w", value, ui.ErrTemplate)
		}

		style.Set |= ui.StyleTransitionDuration
		style.TransitionDuration = uint16(parsed)
	case "transition-delay":
		if parsed > ui.MaxTransitionMilliseconds {
			return fmt.Errorf("invalid transition-delay value %q: %w", value, ui.ErrTemplate)
		}

		style.Set2 |= ui.Style2TransitionDelay
		style.TransitionDelay = uint16(parsed)
	case "left":
		style.Set2 |= ui.Style2Left
		style.Left = uint16(parsed)
	case "right":
		style.Set2 |= ui.Style2Right
		style.Right = uint16(parsed)
	case "top":
		style.Set2 |= ui.Style2Top
		style.Top = uint16(parsed)
	case "bottom":
		style.Set2 |= ui.Style2Bottom
		style.Bottom = uint16(parsed)
	case "icon-size":
		style.Set2 |= ui.Style2IconSize
		style.IconSize = uint16(parsed)
	case "icon-gap":
		style.Set2 |= ui.Style2IconGap
		style.IconGap = uint16(parsed)
	default:
		return fmt.Errorf("unknown style property %s: %w", property, ui.ErrTemplate)
	}

	return nil
}

func parseColor(value string) (uint32, error) {
	hex := strings.TrimPrefix(value, "#")
	if len(hex) == rgbHexLength {
		hex += "ff"
	}

	if len(hex) != 8 || len(value) == len(hex) {
		return 0, fmt.Errorf("invalid color %q: %w", value, ui.ErrTemplate)
	}

	parsed, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid color %q: %w", value, ui.ErrTemplate)
	}

	return uint32(parsed), nil
}

//nolint:gocognit,gocyclo,maintidx // Explicit field merging preserves bounded responsive overrides.
func mergeStyle(base, override ui.Style) ui.Style {
	if override.Set2&ui.Style2Width != 0 {
		base.Width = override.Width
	}

	if override.Set2&ui.Style2Height != 0 {
		base.Height = override.Height
	}

	if override.Set&ui.StyleBackground != 0 {
		base.Background = override.Background
	}

	if override.Set&ui.StyleBackgroundHover != 0 {
		base.BackgroundHover = override.BackgroundHover
	}

	if override.Set&ui.StyleBackgroundPressed != 0 {
		base.BackgroundPressed = override.BackgroundPressed
	}

	if override.Set&ui.StyleColor != 0 {
		base.Color = override.Color
	}

	if override.Set&ui.StylePadding != 0 {
		base.Padding = override.Padding
	}

	if override.Set&ui.StyleGap != 0 {
		base.Gap = override.Gap
	}

	if override.Set&ui.StyleFontSize != 0 {
		base.FontSize = override.FontSize
	}

	if override.Set&ui.StyleMinHeight != 0 {
		base.MinHeight = override.MinHeight
	}

	if override.Set&ui.StyleMinWidth != 0 {
		base.MinWidth = override.MinWidth
	}

	if override.Set&ui.StyleMaxWidth != 0 {
		base.MaxWidth = override.MaxWidth
	}

	if override.Set&ui.StyleMaxHeight != 0 {
		base.MaxHeight = override.MaxHeight
	}

	if override.Set&ui.StyleFlexDirection != 0 {
		base.FlexDirection = override.FlexDirection
	}

	if override.Set&ui.StyleAlignItems != 0 {
		base.AlignItems = override.AlignItems
	}

	if override.Set&ui.StyleJustifyContent != 0 {
		base.JustifyContent = override.JustifyContent
	}

	if override.Set&ui.StyleFlexGrow != 0 {
		base.FlexGrow = override.FlexGrow
	}

	if override.Set&ui.StyleMargin != 0 {
		base.Margin = override.Margin
	}

	if override.Set&ui.StyleOverflow != 0 {
		base.Overflow = override.Overflow
	}

	if override.Set&ui.StyleTransitionDuration != 0 {
		base.TransitionDuration = override.TransitionDuration
	}

	if override.Set&ui.StyleTransitionEnter != 0 {
		base.TransitionEnter = override.TransitionEnter
	}

	if override.Set&ui.StyleTransitionExit != 0 {
		base.TransitionExit = override.TransitionExit
	}

	if override.Set&ui.StyleTextAlign != 0 {
		base.TextAlign = override.TextAlign
	}

	if override.Set&ui.StyleFontFamily != 0 {
		base.FontFamily = override.FontFamily
	}

	if override.Set&ui.StyleContentImage != 0 {
		base.ContentImage = override.ContentImage
	}

	if override.Set&ui.StyleBackgroundDisabled != 0 {
		base.BackgroundDisabled = override.BackgroundDisabled
	}

	if override.Set&ui.StyleColorHover != 0 {
		base.ColorHover = override.ColorHover
	}

	if override.Set&ui.StyleColorPressed != 0 {
		base.ColorPressed = override.ColorPressed
	}

	if override.Set&ui.StyleColorDisabled != 0 {
		base.ColorDisabled = override.ColorDisabled
	}

	if override.Set&ui.StyleBackgroundImage != 0 {
		base.BackgroundImage = override.BackgroundImage
	}

	if override.Set&ui.StyleBackgroundImageHover != 0 {
		base.BackgroundImageHover = override.BackgroundImageHover
	}

	if override.Set&ui.StyleBackgroundImagePressed != 0 {
		base.BackgroundImagePressed = override.BackgroundImagePressed
	}

	if override.Set&ui.StyleBackgroundImageDisabled != 0 {
		base.BackgroundImageDisabled = override.BackgroundImageDisabled
	}

	if override.Set2&ui.Style2BackgroundFocus != 0 {
		base.BackgroundFocus = override.BackgroundFocus
	}

	if override.Set2&ui.Style2BackgroundImageFocus != 0 {
		base.BackgroundImageFocus = override.BackgroundImageFocus
	}

	if override.Set2&ui.Style2ColorFocus != 0 {
		base.ColorFocus = override.ColorFocus
	}

	if override.Set2&ui.Style2TransitionDelay != 0 {
		base.TransitionDelay = override.TransitionDelay
	}

	if override.Set2&ui.Style2TransitionEasing != 0 {
		base.TransitionEasing = override.TransitionEasing
	}

	if override.Set2&ui.Style2Position != 0 {
		base.Position = override.Position
	}

	if override.Set2&ui.Style2Left != 0 {
		base.Left = override.Left
	}

	if override.Set2&ui.Style2Right != 0 {
		base.Right = override.Right
	}

	if override.Set2&ui.Style2Top != 0 {
		base.Top = override.Top
	}

	if override.Set2&ui.Style2Bottom != 0 {
		base.Bottom = override.Bottom
	}

	if override.Set2&ui.Style2ImageFit != 0 {
		base.ImageFit = override.ImageFit
	}

	if override.Set2&ui.Style2Tint != 0 {
		base.Tint = override.Tint
	}

	if override.Set2&ui.Style2Icon != 0 {
		base.Icon = override.Icon
	}

	if override.Set2&ui.Style2IconSize != 0 {
		base.IconSize = override.IconSize
	}

	if override.Set2&ui.Style2IconPosition != 0 {
		base.IconPosition = override.IconPosition
	}

	if override.Set2&ui.Style2IconGap != 0 {
		base.IconGap = override.IconGap
	}

	base.Set |= override.Set
	base.Set2 |= override.Set2

	return base
}
