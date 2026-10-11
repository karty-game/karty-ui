// Package ui defines bounded author-owned presentation assets shared by the
// build pipeline and host. It has no dependency on a rendering library.
package ui

import (
	"errors"
	"unicode/utf8"
)

const (
	MaxAssetBytes                    = 64 * 1024
	MaxElements                      = 64
	MaxTextBytes                     = 4096
	MaxNameBytes                     = 128
	MaxRows                          = 256
	MaxTransitionMilliseconds        = 2000
	BackElementID             uint32 = MaxElements + 1
	AssetPrefix                      = "karty.ui/"
	FontAssetPrefix                  = "karty.font/"
	SectionName                      = "karty.ui.v1"
	SchemaComposition         uint32 = 2
	SchemaStyle               uint32 = 3
	SchemaImageStyle          uint32 = 4
	SchemaLayout              uint32 = 5
	SchemaResponsive          uint32 = 6
	SchemaControlTransition   uint32 = 7
	SchemaVisualHierarchy     uint32 = 8
	SchemaInteractionPolish   uint32 = 9
	SchemaWidgets             uint32 = 10
	SchemaSizing              uint32 = 11
	MinResponsiveWidth               = 240
	MaxResponsiveWidth               = 1024
)

const (
	StyleBackground uint32 = 1 << iota
	StyleBackgroundHover
	StyleBackgroundPressed
	StyleColor
	StylePadding
	StyleGap
	StyleFontSize
	StyleMinHeight
	StyleBackgroundImage
	StyleBackgroundImageHover
	StyleBackgroundImagePressed
	StyleBackgroundDisabled
	StyleBackgroundImageDisabled
	StyleColorHover
	StyleColorPressed
	StyleColorDisabled
	StyleFlexDirection
	StyleAlignItems
	StyleMinWidth
	StyleMaxWidth
	StyleMaxHeight
	StyleJustifyContent
	StyleFlexGrow
	StyleMargin
	StyleOverflow
	StyleTransitionDuration
	StyleTransitionEnter
	StyleTransitionExit
	StyleTextAlign
	StyleFontFamily
	StyleContentImage
)

const (
	Style2BackgroundFocus uint32 = 1 << iota
	Style2BackgroundImageFocus
	Style2ColorFocus
	Style2TransitionDelay
	Style2TransitionEasing
	Style2Position
	Style2Left
	Style2Right
	Style2Top
	Style2Bottom
	Style2ImageFit
	Style2Tint
	Style2Icon
	Style2IconSize
	Style2IconPosition
	Style2IconGap
	Style2Width
	Style2Height
)

const (
	FlexDirectionColumn uint8 = iota + 1
	FlexDirectionRow
)

const (
	AlignItemsStart uint8 = iota + 1
	AlignItemsCenter
	AlignItemsEnd
	AlignItemsStretch
)

const (
	JustifyContentStart uint8 = iota + 1
	JustifyContentCenter
	JustifyContentEnd
	JustifyContentSpaceBetween
)

const (
	OverflowVisible uint8 = iota + 1
	OverflowScroll
)

const (
	TransitionNone uint8 = iota
	TransitionLeft
	TransitionRight
	TransitionTop
	TransitionBottom
)

const (
	TransitionEasingLinear uint8 = iota + 1
	TransitionEasingOut
	TransitionEasingInOut
)

const (
	ImageFitStretch uint8 = iota + 1
	ImageFitContain
	ImageFitCover
)

const (
	IconPositionStart uint8 = iota + 1
	IconPositionEnd
)

const PositionOverlay uint8 = 1

const (
	TextAlignStart uint8 = iota + 1
	TextAlignCenter
	TextAlignEnd
)

const (
	FontFamilyBody uint8 = iota + 1
	FontFamilyDisplay
	FontFamilyMono
)

const (
	ImageScopeGame uint8 = iota + 1
	ImageScopeLevel
)

var ErrTemplate = errors.New("invalid UI template")

// Template version 1 is a vertical panel of named labels, buttons, and lists.
// A modal panel captures game input. Schema 9 also permits actionable nonmodal
// HUD controls; older nonmodal templates retain their noninteractive contract.
type Template struct {
	Version            uint32
	Modal              bool
	Back               bool
	Panel              Style
	ResponsiveMaxWidth uint16
	ResponsivePanel    Style
	Elements           []Element
}

type Element struct {
	ID          uint32
	Parent      uint32
	Name        string
	Kind        string
	Text        string
	Action      uint32
	Value       string
	Min         int32
	Max         int32
	Placeholder string
	Tooltip     string
	Style       Style
	// ResponsiveStyle is the fully resolved style selected at or below the
	// template's ResponsiveMaxWidth. A zero Set inherits Style unchanged.
	ResponsiveStyle Style
}

// Style is a compact, compiler-resolved presentation value. Set distinguishes
// an authored zero/transparent value from an omitted property.
type Style struct {
	Set                     uint32
	Set2                    uint32
	Background              uint32
	BackgroundHover         uint32
	BackgroundPressed       uint32
	Color                   uint32
	Padding                 uint16
	Gap                     uint16
	FontSize                uint16
	MinHeight               uint16
	BackgroundImage         Image
	BackgroundImageHover    Image
	BackgroundImagePressed  Image
	BackgroundDisabled      uint32
	BackgroundImageDisabled Image
	ColorHover              uint32
	ColorPressed            uint32
	ColorDisabled           uint32
	FlexDirection           uint8
	AlignItems              uint8
	MinWidth                uint16
	MaxWidth                uint16
	MaxHeight               uint16
	JustifyContent          uint8
	FlexGrow                uint8
	Margin                  uint16
	Overflow                uint8
	TransitionDuration      uint16
	TransitionEnter         uint8
	TransitionExit          uint8
	TextAlign               uint8
	FontFamily              uint8
	ContentImage            Image
	BackgroundFocus         uint32
	BackgroundImageFocus    Image
	ColorFocus              uint32
	TransitionDelay         uint16
	TransitionEasing        uint8
	Position                uint8
	Left                    uint16
	Right                   uint16
	Top                     uint16
	Bottom                  uint16
	ImageFit                uint8
	Tint                    uint32
	Icon                    Image
	IconSize                uint16
	IconPosition            uint8
	IconGap                 uint16
	Width                   Length
	Height                  Length
}

// Image is a compiler-resolved nine-slice reference. Game images use a logical
// name; level images use the stable numeric ID from their owning .kld.
type Image struct {
	Scope   uint8
	Name    string
	AssetID uint32
	Top     uint16
	Right   uint16
	Bottom  uint16
	Left    uint16
}

func Decode(data []byte) (Template, error) {
	return decode(data, false)
}

// DecodeComposition accepts all versioned retained-tree presentation schemas.
func DecodeComposition(data []byte) (Template, error) {
	return decode(data, true)
}

func decode(data []byte, composition bool) (Template, error) {
	result, err := decodeBinary(data)
	if err != nil {
		return Template{}, err
	}

	if err := result.validate(composition); err != nil {
		return Template{}, err
	}

	return result, nil
}

func (template Template) Validate() error {
	return template.validate(false)
}

// ValidateComposition also accepts composition, style, layout, and responsive schemas.
// Legacy packaging continues using Validate, which accepts schema 1 only.
func (template Template) ValidateComposition() error {
	return template.validate(true)
}

// ValidateStyle checks a resolved style against a widget's complete current
// presentation contract. Authoring compilers may use this to omit invalid
// declarations; serialized templates still undergo strict versioned validation.
func ValidateStyle(style Style, kind string) error {
	if !validStyle(style, kind, SchemaSizing) {
		return ErrTemplate
	}

	return nil
}

func (template Template) validate(composition bool) error {
	if !validVersion(template.Version, composition) || len(template.Elements) == 0 || len(template.Elements) > MaxElements {
		return ErrTemplate
	}

	if template.Back && (!template.Modal || template.Version < SchemaInteractionPolish) {
		return ErrTemplate
	}

	if !validStyle(template.Panel, "root", template.Version) ||
		(template.Version < SchemaStyle && template.Panel.Set|template.Panel.Set2 != 0) ||
		!validResponsive(template) {
		return ErrTemplate
	}

	state := validationState{
		ids:    make(map[uint32]bool, len(template.Elements)),
		names:  make(map[string]bool, len(template.Elements)),
		panels: make(map[uint32]uint32),
		kinds:  make(map[uint32]string),
	}
	for _, element := range template.Elements {
		if !state.accept(element, template.Version, template.Modal, composition) {
			return ErrTemplate
		}
	}

	return template.validateTabs()
}

type validationState struct {
	ids    map[uint32]bool
	names  map[string]bool
	panels map[uint32]uint32
	kinds  map[uint32]string
}

//nolint:cyclop // Identity, ancestry, version and presentation bounds are checked before accepting an element.
func (state *validationState) accept(element Element, version uint32, modal, composition bool) bool {
	if element.ID == 0 || state.ids[element.ID] || state.names[element.Name] || !validName(element.Name) ||
		len(element.Text) > MaxTextBytes || !utf8.ValidString(element.Text) {
		return false
	}

	state.ids[element.ID], state.names[element.Name] = true, true
	depth := uint32(1)

	if element.Parent != 0 {
		parentDepth, exists := state.panels[element.Parent]
		if version < SchemaLayout || !exists || parentDepth >= 15 {
			return false
		}

		depth = parentDepth + 1
	}

	if isContainerKind(element.Kind) {
		if version < SchemaLayout {
			return false
		}

		state.panels[element.ID] = depth
	}

	parentKind := state.kinds[element.Parent]

	state.kinds[element.ID] = element.Kind
	if (element.Kind == "tab") != (parentKind == "tabs") {
		return false
	}

	return validWidgetFields(element, version) && validElementKind(element, modal, composition && version >= SchemaComposition, version) &&
		(element.Kind != "image" || version >= SchemaVisualHierarchy) &&
		validStyle(element.Style, element.Kind, version) && validResponsiveStyle(element.ResponsiveStyle, element.Kind, version) &&
		(version >= SchemaStyle || element.Style.Set|element.Style.Set2 == 0)
}

func validVersion(version uint32, composition bool) bool {
	return version == 1 || (composition && version >= SchemaComposition && version <= SchemaSizing)
}

func validResponsive(template Template) bool {
	hasStyles := template.ResponsivePanel.Set|template.ResponsivePanel.Set2 != 0
	for _, element := range template.Elements {
		hasStyles = hasStyles || element.ResponsiveStyle.Set|element.ResponsiveStyle.Set2 != 0
	}

	if template.ResponsiveMaxWidth == 0 {
		return !hasStyles
	}

	return hasStyles && template.Version >= SchemaResponsive && template.ResponsiveMaxWidth >= MinResponsiveWidth &&
		template.ResponsiveMaxWidth <= MaxResponsiveWidth &&
		validResponsiveStyle(template.ResponsivePanel, "root", template.Version)
}

func validResponsiveStyle(style Style, kind string, version uint32) bool {
	return style.Set|style.Set2 == 0 || (version >= SchemaResponsive && validStyle(style, kind, version))
}

//nolint:cyclop,funlen,gocyclo,gocognit,maintidx // Explicit per-widget allowlists keep the serialized schema auditable.
func validStyle(style Style, kind string, version uint32) bool {
	if !validWidgetStyle(kind, style) {
		return false
	}

	if !validSizing(style, kind, version) {
		return false
	}

	kind = widgetStyleKind(kind)

	const all = StyleBackground | StyleBackgroundHover | StyleBackgroundPressed | StyleColor |
		StylePadding | StyleGap | StyleFontSize | StyleMinHeight | StyleBackgroundImage |
		StyleBackgroundImageHover | StyleBackgroundImagePressed | StyleBackgroundDisabled |
		StyleBackgroundImageDisabled | StyleColorHover | StyleColorPressed | StyleColorDisabled |
		StyleFlexDirection | StyleAlignItems | StyleMinWidth | StyleMaxWidth | StyleMaxHeight |
		StyleJustifyContent | StyleFlexGrow | StyleMargin | StyleOverflow | StyleTransitionDuration |
		StyleTransitionEnter | StyleTransitionExit | StyleTextAlign | StyleFontFamily | StyleContentImage

	if style.Set&^all != 0 || !validStyleValues(style) {
		return false
	}

	const all2 = Style2BackgroundFocus | Style2BackgroundImageFocus | Style2ColorFocus |
		Style2TransitionDelay | Style2TransitionEasing | Style2Position |
		Style2Left | Style2Right | Style2Top | Style2Bottom | Style2ImageFit | Style2Tint |
		Style2Icon | Style2IconSize | Style2IconPosition | Style2IconGap | Style2Width | Style2Height
	if style.Set2&^all2 != 0 {
		return false
	}

	var allowed uint32

	switch kind {
	case "root":
		allowed = StyleBackground | StyleBackgroundImage | StylePadding | StyleGap |
			StyleFlexDirection | StyleAlignItems | StyleMinWidth | StyleMinHeight | StyleMaxWidth | StyleMaxHeight |
			StyleJustifyContent | StyleFlexGrow | StyleMargin | StyleOverflow | StyleTransitionDuration |
			StyleTransitionEnter | StyleTransitionExit
	case "panel":
		allowed = StyleBackground | StyleBackgroundImage | StylePadding | StyleGap |
			StyleFlexDirection | StyleAlignItems | StyleMinWidth | StyleMinHeight | StyleMaxWidth | StyleMaxHeight |
			StyleJustifyContent | StyleFlexGrow | StyleMargin | StyleOverflow | StyleTransitionDuration |
			StyleTransitionEnter | StyleTransitionExit
	case "label":
		allowed = StyleColor | StyleFontSize | StyleMinWidth | StyleMinHeight | StyleMaxWidth | StyleMaxHeight |
			StyleFlexGrow | StyleMargin | StyleTextAlign | StyleFontFamily
	case "image":
		allowed = StyleContentImage | StyleMinWidth | StyleMinHeight | StyleMaxWidth | StyleMaxHeight |
			StyleFlexGrow | StyleMargin
	case "button", "list":
		allowed = StyleBackground | StyleBackgroundHover | StyleBackgroundPressed | StyleBackgroundImage |
			StyleBackgroundImageHover | StyleBackgroundImagePressed | StyleBackgroundDisabled |
			StyleBackgroundImageDisabled | StyleColor | StyleColorHover | StyleColorPressed |
			StyleColorDisabled | StyleFontSize | StyleMinWidth | StyleMinHeight | StyleMaxWidth | StyleMaxHeight |
			StyleFlexGrow | StyleMargin | StyleTransitionDuration | StyleTextAlign | StyleFontFamily
	case "slot":
		allowed = 0
	default:
		return false
	}

	if style.Set&^allowed != 0 {
		return false
	}

	if style.Set2&(Style2BackgroundFocus|Style2BackgroundImageFocus|Style2ColorFocus) != 0 &&
		kind != "button" && kind != "list" {
		return false
	}

	if style.Set2&(Style2TransitionDelay|Style2TransitionEasing) != 0 &&
		kind != "root" && kind != "panel" && kind != "button" && kind != "list" {
		return false
	}

	const overlay = Style2Position | Style2Left | Style2Right | Style2Top | Style2Bottom
	if style.Set2&overlay != 0 && (kind == "root" || kind == "slot") {
		return false
	}

	if style.Set2&(Style2Left|Style2Right|Style2Top|Style2Bottom) != 0 && style.Set2&Style2Position == 0 {
		return false
	}

	if style.Set2&(Style2ImageFit) != 0 && kind != "image" {
		return false
	}

	if style.Set2&(Style2Icon|Style2IconSize|Style2IconPosition|Style2IconGap) != 0 &&
		kind != "button" && kind != "list" {
		return false
	}

	if style.Set2&Style2Tint != 0 && kind != "image" && kind != "button" && kind != "list" {
		return false
	}

	if style.Set2&(Style2IconSize|Style2IconPosition|Style2IconGap) != 0 && style.Set2&Style2Icon == 0 {
		return false
	}

	if kind == "panel" && style.Set&StyleTransitionDuration != 0 &&
		style.Set&(StyleTransitionEnter|StyleTransitionExit) == 0 {
		return false
	}

	if kind == "panel" && style.Set&StyleOverflow != 0 && style.Overflow == OverflowScroll &&
		style.Set&(StyleTransitionEnter|StyleTransitionExit) != 0 {
		return false
	}

	if !validStyleSchema(style, kind, version) {
		return false
	}

	images := []struct {
		flag  uint32
		value Image
	}{
		{StyleBackgroundImage, style.BackgroundImage},
		{StyleBackgroundImageHover, style.BackgroundImageHover},
		{StyleBackgroundImagePressed, style.BackgroundImagePressed},
		{StyleBackgroundImageDisabled, style.BackgroundImageDisabled},
		{StyleContentImage, style.ContentImage},
	}
	for _, image := range images {
		set := style.Set&image.flag != 0
		if set != validImage(image.value) || (set && version < 4) {
			return false
		}
	}

	if set := style.Set2&Style2BackgroundImageFocus != 0; set != validImage(style.BackgroundImageFocus) ||
		(set && version < SchemaInteractionPolish) {
		return false
	}

	if set := style.Set2&Style2Icon != 0; set != validImage(style.Icon) ||
		(set && version < SchemaInteractionPolish) {
		return false
	}

	return true
}

//nolint:cyclop // Explicit field bounds keep the serialized schema auditable.
func validStyleValues(style Style) bool {
	const (
		maxSpacing  = 256
		maxFontSize = 128
		maxSize     = 2048
		maxFlexGrow = 16
	)

	if style.Padding > maxSpacing || style.Gap > maxSpacing || style.Margin > maxSpacing ||
		style.FontSize > maxFontSize || style.MinHeight > maxSpacing || style.MinWidth > maxSize ||
		style.MaxWidth > maxSize || style.MaxHeight > maxSize || style.TransitionDuration > MaxTransitionMilliseconds ||
		style.TransitionDelay > MaxTransitionMilliseconds || style.Left > maxSize || style.Right > maxSize ||
		style.Top > maxSize || style.Bottom > maxSize ||
		style.IconSize > maxSpacing || style.IconGap > maxSpacing || style.FlexGrow > maxFlexGrow {
		return false
	}

	if style.Set&(StyleTransitionEnter|StyleTransitionExit) != 0 &&
		(style.Set&StyleTransitionDuration == 0 || style.TransitionDuration == 0) {
		return false
	}

	if style.Set2&(Style2TransitionDelay|Style2TransitionEasing) != 0 &&
		(style.Set&StyleTransitionDuration == 0 || style.TransitionDuration == 0) {
		return false
	}

	return validStyleEnums(style)
}

//nolint:cyclop,gocyclo // Every optional serialized enum is checked explicitly.
func validStyleEnums(style Style) bool {
	directionValid := style.FlexDirection == FlexDirectionColumn || style.FlexDirection == FlexDirectionRow
	alignmentValid := style.AlignItems >= AlignItemsStart && style.AlignItems <= AlignItemsStretch
	justifyValid := style.JustifyContent >= JustifyContentStart && style.JustifyContent <= JustifyContentSpaceBetween
	overflowValid := style.Overflow == OverflowVisible || style.Overflow == OverflowScroll
	enterValid := style.TransitionEnter >= TransitionLeft && style.TransitionEnter <= TransitionBottom
	exitValid := style.TransitionExit >= TransitionLeft && style.TransitionExit <= TransitionBottom
	textAlignValid := style.TextAlign >= TextAlignStart && style.TextAlign <= TextAlignEnd
	fontFamilyValid := style.FontFamily >= FontFamilyBody && style.FontFamily <= FontFamilyMono
	easingValid := style.TransitionEasing >= TransitionEasingLinear && style.TransitionEasing <= TransitionEasingInOut
	positionValid := style.Position == PositionOverlay
	imageFitValid := style.ImageFit >= ImageFitStretch && style.ImageFit <= ImageFitCover
	iconPositionValid := style.IconPosition >= IconPositionStart && style.IconPosition <= IconPositionEnd

	return (style.Set&StyleFlexDirection == 0 || directionValid) &&
		(style.Set&StyleAlignItems == 0 || alignmentValid) &&
		(style.Set&StyleJustifyContent == 0 || justifyValid) &&
		(style.Set&StyleOverflow == 0 || overflowValid) &&
		(style.Set&StyleTransitionEnter == 0 || enterValid) &&
		(style.Set&StyleTransitionExit == 0 || exitValid) &&
		(style.Set&StyleTextAlign == 0 || textAlignValid) &&
		(style.Set&StyleFontFamily == 0 || fontFamilyValid) &&
		(style.Set2&Style2TransitionEasing == 0 || easingValid) &&
		(style.Set2&Style2Position == 0 || positionValid) &&
		(style.Set2&Style2ImageFit == 0 || imageFitValid) &&
		(style.Set2&Style2IconPosition == 0 || iconPositionValid)
}

func validStyleSchema(style Style, kind string, version uint32) bool {
	const interaction = StyleBackgroundDisabled | StyleBackgroundImageDisabled |
		StyleColorHover | StyleColorPressed | StyleColorDisabled

	const layout = StyleFlexDirection | StyleAlignItems | StyleMinWidth | StyleMaxWidth | StyleMaxHeight |
		StyleJustifyContent | StyleFlexGrow | StyleMargin | StyleOverflow | StyleTransitionDuration

	if style.Set&(StyleTransitionEnter|StyleTransitionExit) != 0 && version < SchemaControlTransition {
		return false
	}

	if style.Set&StyleTransitionDuration != 0 && kind != "root" && version < SchemaControlTransition {
		return false
	}

	if style.Set&(StyleTextAlign|StyleFontFamily|StyleContentImage) != 0 && version < SchemaVisualHierarchy {
		return false
	}

	if style.Set2 != 0 && version < SchemaInteractionPolish {
		return false
	}

	if version >= SchemaLayout {
		return true
	}

	return style.Set&(interaction|layout) == 0 &&
		(style.Set&StyleMinHeight == 0 || kind == "button" || kind == "list")
}

func validImage(image Image) bool {
	if image.Top > 256 || image.Right > 256 || image.Bottom > 256 || image.Left > 256 {
		return false
	}

	switch image.Scope {
	case ImageScopeGame:
		return image.AssetID == 0 && len(image.Name) > 0 && len(image.Name) <= MaxNameBytes && utf8.ValidString(image.Name)
	case ImageScopeLevel:
		return image.AssetID != 0 && image.Name == ""
	default:
		return false
	}
}

func validElementKind(element Element, modal, slots bool, version uint32) bool {
	switch element.Kind {
	case "checkbox", "combo", "input", "slider", "tabs":
		return version >= SchemaWidgets && element.Action != 0
	case "tab":
		return version >= SchemaWidgets && element.Action == 0
	case "panel":
		return slots && element.Action == 0 && element.Text == ""
	case "slot":
		return slots && element.Action == 0 && element.Text == ""
	case "label":
		return element.Action == 0
	case "image":
		return slots && element.Action == 0 && element.Text == "" && element.Style.Set&StyleContentImage != 0
	case "button", "list":
		return element.Action != 0 && (modal || version >= SchemaInteractionPolish)
	default:
		return false
	}
}

func validName(name string) bool {
	if len(name) == 0 || len(name) > MaxNameBytes {
		return false
	}

	for _, character := range name {
		if character != '_' && (character < 'a' || character > 'z') && (character < '0' || character > '9') {
			return false
		}
	}

	return true
}

// Encode emits a bounded KUI1 binary asset with semantic schema version 1.
func Encode(template Template) ([]byte, error) {
	return encode(template, false)
}

func EncodeComposition(template Template) ([]byte, error) { return encode(template, true) }

func encode(template Template, composition bool) ([]byte, error) {
	if err := template.validate(composition); err != nil {
		return nil, err
	}

	return encodeBinary(template)
}
