package ui

import (
	"strconv"
	"unicode/utf8"
)

// ValidValue checks a control's canonical wire value. Selection membership is
// checked by the host against the complete staged tree and its current rows.
func (element Element) ValidValue(value string) bool {
	if len(value) > MaxTextBytes || !utf8.ValidString(value) {
		return false
	}

	switch element.Kind {
	case "checkbox":
		return value == "true" || value == "false"
	case "slider":
		number, err := strconv.ParseInt(value, 10, 32)

		return err == nil && strconv.FormatInt(number, 10) == value && number >= int64(element.Min) && number <= int64(element.Max)
	case "combo", "tabs":
		number, err := strconv.ParseUint(value, 10, 32)

		return err == nil && strconv.FormatUint(number, 10) == value
	default:
		return true
	}
}

//nolint:cyclop // Explicit bounds and per-control field ownership are validated together.
func validWidgetFields(element Element, version uint32) bool {
	for _, value := range []string{element.Value, element.Placeholder, element.Tooltip} {
		if len(value) > MaxTextBytes || !utf8.ValidString(value) {
			return false
		}
	}

	if version < SchemaWidgets {
		return element.Value == "" && element.Min == 0 && element.Max == 0 && element.Placeholder == "" && element.Tooltip == ""
	}

	if element.Kind != "input" && element.Placeholder != "" {
		return false
	}

	if element.Kind != "slider" && (element.Min != 0 || element.Max != 0) {
		return false
	}

	switch element.Kind {
	case "slider":
		return element.Min >= -1000000 && element.Max <= 1000000 && element.Min < element.Max && element.ValidValue(element.Value) &&
			element.Text == ""
	case "checkbox":
		return element.ValidValue(element.Value)
	case "combo", "tabs":
		return element.ValidValue(element.Value) && element.Text == ""
	case "input":
		return element.Text == ""
	default:
		return element.Value == "" && (element.Kind != "slot" || element.Tooltip == "")
	}
}

func (template Template) validateTabs() error {
	for _, element := range template.Elements {
		if element.Kind != "tabs" {
			continue
		}

		count := uint64(0)

		for _, tab := range template.Elements {
			if tab.Parent == element.ID {
				count++
			}
		}

		selected, err := strconv.ParseUint(element.Value, 10, 32)
		if err != nil || selected >= count {
			return ErrTemplate
		}
	}

	return nil
}

func isContainerKind(kind string) bool { return kind == "panel" || kind == "tabs" || kind == "tab" }
func widgetStyleKind(kind string) string {
	switch kind {
	case "checkbox", "combo", "input", "slider":
		return "button"
	case "tabs", "tab":
		return "panel"
	default:
		return kind
	}
}

func validWidgetStyle(kind string, style Style) bool {
	switch kind {
	case "checkbox", "combo", "input", "slider":
		return style.Set&StyleTransitionDuration == 0 &&
			style.Set2 & ^(Style2Position|Style2Left|Style2Right|Style2Top|Style2Bottom|Style2Width|Style2Height) == 0
	case "tabs":
		const layout = StyleMinWidth | StyleMaxWidth | StyleMinHeight | StyleMaxHeight | StyleMargin | StyleFlexGrow

		return style.Set & ^layout == 0 &&
			style.Set2 & ^(Style2Position|Style2Left|Style2Right|Style2Top|Style2Bottom|Style2Width|Style2Height) == 0
	case "tab":
		return style.Set&(StyleOverflow|StyleTransitionDuration|StyleTransitionEnter|StyleTransitionExit) == 0
	default:
		return true
	}
}
