package uicompiler

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	ui "github.com/karty-game/karty-ui/schema"
)

const (
	maxStyleWarnings     = 128
	maxStyleWarningBytes = 2048
)

type styleSource struct {
	lineStarts                    []int
	source, text, original, scope string
	start                         int
	spans                         []styleSpan
	warnings                      []Diagnostic
}

type styleDeclaration struct {
	source, selector, property, value string
	style                             ui.Style
	location                          SourceLocation
}

// StyleWarnings returns build-time diagnostics for ignored styles. The compiler
// does not write to stderr; callers decide where warnings should be displayed.
func (component *Component) StyleWarnings() []string {
	result := make([]string, len(component.styleWarnings))
	for index, warning := range component.styleWarnings {
		result[index] = warning.String()
	}

	return result
}

func appendStyleWarning(warnings *[]Diagnostic, diagnostic Diagnostic) {
	diagnostic.Message = boundedStyleWarningText(diagnostic.Message)

	if slices.Contains(*warnings, diagnostic) {
		return
	}

	if len(*warnings) == maxStyleWarnings {
		*warnings = append(*warnings, Diagnostic{Message: "additional style warnings omitted"})

		return
	}

	if len(*warnings) < maxStyleWarnings {
		*warnings = append(*warnings, diagnostic)
	}
}

func boundedStyleWarningText(message string) string {
	if len(message) <= maxStyleWarningBytes {
		return message
	}

	message = message[:maxStyleWarningBytes]
	for !utf8.ValidString(message) {
		message = message[:len(message)-1]
	}

	return message + "..."
}

func (declaration styleDeclaration) warn(warnings *[]Diagnostic, reason string) {
	reason = strings.TrimSuffix(reason, ": "+ui.ErrTemplate.Error())
	appendStyleWarning(warnings, Diagnostic{SourceLocation: declaration.location,
		Message: fmt.Sprintf("style %s: %s: %q ignored: %s", declaration.selector, declaration.property, declaration.value, reason)})
}

// Supply only prerequisite fields while checking one property. These dummy
// values are never emitted; final styles must satisfy all real prerequisites.
func stylePropertyValid(style ui.Style, kind string) bool {
	const timing = ui.Style2TransitionDelay | ui.Style2TransitionEasing
	if style.Set&(ui.StyleTransitionEnter|ui.StyleTransitionExit) != 0 || style.Set2&timing != 0 {
		if style.Set&ui.StyleTransitionDuration == 0 {
			style.Set |= ui.StyleTransitionDuration
			style.TransitionDuration = 1
		}
	}

	if kind == "panel" && style.Set&ui.StyleTransitionDuration != 0 && style.Set&(ui.StyleTransitionEnter|ui.StyleTransitionExit) == 0 {
		style.Set |= ui.StyleTransitionEnter
		style.TransitionEnter = ui.TransitionLeft
	}

	if style.Set2&(ui.Style2Left|ui.Style2Right|ui.Style2Top|ui.Style2Bottom) != 0 {
		style.Set2 |= ui.Style2Position
		style.Position = ui.PositionOverlay
	}

	if style.Set2&(ui.Style2IconSize|ui.Style2IconPosition|ui.Style2IconGap) != 0 && style.Set2&ui.Style2Icon == 0 {
		style.Set2 |= ui.Style2Icon
		style.Icon = ui.Image{Scope: ui.ImageScopeGame, Name: "style-validation"}
	}

	return ui.ValidateStyle(style, kind) == nil
}

func (component *Component) mergeStyleDeclarations(base ui.Style, declarations []styleDeclaration, kind string) ui.Style {
	for _, declaration := range declarations {
		if !stylePropertyValid(declaration.style, kind) {
			declaration.warn(&component.styleWarnings, "property is unsupported on "+kind+" or outside its allowed range")

			continue
		}

		candidate := mergeStyle(base, declaration.style)
		if reason := styleConflict(candidate, kind); reason != "" {
			declaration.warn(&component.styleWarnings, reason)

			continue
		}

		base = candidate

		if component.styleLocations == nil {
			component.styleLocations = map[int]map[string]SourceLocation{}
		}

		locations := component.styleLocations[component.currentStyleIndex]
		if locations == nil {
			locations = map[string]SourceLocation{}
			component.styleLocations[component.currentStyleIndex] = locations
		}

		locations[declaration.property] = declaration.location
	}

	return base
}

func styleConflict(style ui.Style, kind string) string {
	if style.Set&(ui.StyleMinWidth|ui.StyleMaxWidth) == ui.StyleMinWidth|ui.StyleMaxWidth && style.MinWidth > style.MaxWidth {
		return "min-width exceeds max-width"
	}

	if style.Set&(ui.StyleMinHeight|ui.StyleMaxHeight) == ui.StyleMinHeight|ui.StyleMaxHeight && style.MinHeight > style.MaxHeight {
		return "min-height exceeds max-height"
	}

	if kind == "panel" && style.Set&ui.StyleOverflow != 0 && style.Overflow == ui.OverflowScroll &&
		style.Set&(ui.StyleTransitionEnter|ui.StyleTransitionExit) != 0 {
		return "scroll panels do not support slide transitions"
	}

	return ""
}

func (component *Component) dependencyWarning(kind, property, reason string) {
	location := component.currentStyleLocation
	for name := range strings.SplitSeq(property, "/") {
		if found, exists := component.styleLocations[component.currentStyleIndex][name]; exists {
			location = found

			break
		}

		if component.currentStyleIndex < 0 {
			baseIndex := -component.currentStyleIndex - 1
			if found, exists := component.styleLocations[baseIndex][name]; exists {
				location = found

				break
			}
		}
	}

	appendStyleWarning(
		&component.styleWarnings,
		Diagnostic{SourceLocation: location, Message: fmt.Sprintf("%s style %s ignored: %s", kind, property, reason)},
	)
}

func (component *Component) cleanStyleDependencies(style ui.Style, kind string) ui.Style {
	if style.Set2&ui.Style2Position == 0 {
		for _, field := range []struct {
			flag  uint32
			value *uint16
			name  string
		}{
			{ui.Style2Left, &style.Left, "left"}, {ui.Style2Right, &style.Right, "right"},
			{ui.Style2Top, &style.Top, "top"}, {ui.Style2Bottom, &style.Bottom, "bottom"},
		} {
			if style.Set2&field.flag != 0 {
				component.dependencyWarning(kind, field.name, "requires position: absolute")
				style.Set2 &^= field.flag
				*field.value = 0
			}
		}
	}

	if style.Set&ui.StyleTransitionDuration == 0 || style.TransitionDuration == 0 {
		if style.Set&(ui.StyleTransitionEnter|ui.StyleTransitionExit) != 0 {
			component.dependencyWarning(kind, "transition-enter/transition-exit", "requires a positive transition-duration")

			style.Set &^= ui.StyleTransitionEnter | ui.StyleTransitionExit
			style.TransitionEnter, style.TransitionExit = 0, 0
		}

		if style.Set2&(ui.Style2TransitionDelay|ui.Style2TransitionEasing) != 0 {
			component.dependencyWarning(kind, "transition-delay/transition-easing", "requires a positive transition-duration")

			style.Set2 &^= ui.Style2TransitionDelay | ui.Style2TransitionEasing
			style.TransitionDelay, style.TransitionEasing = 0, 0
		}
	}

	if kind == "panel" && style.Set&ui.StyleTransitionDuration != 0 && style.Set&(ui.StyleTransitionEnter|ui.StyleTransitionExit) == 0 {
		component.dependencyWarning(kind, "transition-duration", "requires transition-enter or transition-exit on a nested panel")

		style.Set &^= ui.StyleTransitionDuration
		style.TransitionDuration = 0
		style.Set2 &^= ui.Style2TransitionDelay | ui.Style2TransitionEasing
		style.TransitionDelay, style.TransitionEasing = 0, 0
	}

	if style.Set2&ui.Style2Icon == 0 && style.Set2&(ui.Style2IconSize|ui.Style2IconPosition|ui.Style2IconGap) != 0 {
		component.dependencyWarning(kind, "icon-size/icon-position/icon-gap", "requires an icon")

		style.Set2 &^= ui.Style2IconSize | ui.Style2IconPosition | ui.Style2IconGap
		style.IconSize, style.IconPosition, style.IconGap = 0, 0, 0
	}

	return style
}

func (component *Component) hasResponsiveStyle() bool {
	if component.Template.ResponsivePanel.Set|component.Template.ResponsivePanel.Set2 != 0 {
		return true
	}

	for _, element := range component.Template.Elements {
		if element.ResponsiveStyle.Set|element.ResponsiveStyle.Set2 != 0 {
			return true
		}
	}

	return false
}
