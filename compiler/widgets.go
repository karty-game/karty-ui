package uicompiler

import (
	"encoding/xml"
	"fmt"
	"strconv"

	ui "github.com/karty-game/karty-ui/schema"
)

func isControlContainer(kind string) bool { return kind == "panel" || kind == "tabs" || kind == "tab" }
func isNewWidget(kind string) bool {
	switch kind {
	case "checkbox", "combo", "input", "slider", "tabs", "tab":
		return true
	}

	return false
}
func initializeWidget(element *ui.Element) {
	switch element.Kind {
	case "checkbox":
		element.Value = "false"
	case "combo", "tabs":
		element.Value = "0"
	case "slider":
		element.Value = "0"
		element.Max = 100
	}
}

//nolint:gocognit // Attribute dispatch explicitly pairs each value type with its widget.
func widgetAttribute(element *ui.Element, binding *Binding, attr xml.Attr, expressions map[string]string) (bool, error) {
	expression, dynamic := expressions[attr.Value]
	switch attr.Name.Local {
	case "tooltip", "placeholder", "title", "min", "max":
		if dynamic {
			return true, fmt.Errorf("%s must be literal: %w", attr.Name.Local, ui.ErrTemplate)
		}

		switch attr.Name.Local {
		case "tooltip":
			element.Tooltip = attr.Value
		case "placeholder":
			if element.Kind != "input" {
				return true, ui.ErrTemplate
			}

			element.Placeholder = attr.Value
		case "title":
			if element.Kind != "tab" {
				return true, ui.ErrTemplate
			}

			element.Text = attr.Value
		case "min", "max":
			if element.Kind != "slider" {
				return true, ui.ErrTemplate
			}

			value, err := strconv.ParseInt(attr.Value, 10, 32)
			if err != nil {
				return true, ui.ErrTemplate
			}

			if attr.Name.Local == "min" {
				element.Min = int32(value)
				element.Value = strconv.FormatInt(value, 10)
			} else {
				element.Max = int32(value)
			}
		}

		return true, nil
	case "checked", "value", "selected", "onChange":
		if !dynamic {
			return true, fmt.Errorf("%s requires a Go expression: %w", attr.Name.Local, ui.ErrTemplate)
		}

		if attr.Name.Local == "onChange" {
			binding.Change = expression
			element.Action = element.ID

			return true, nil
		}

		var method string

		switch {
		case element.Kind == "checkbox" && attr.Name.Local == "checked":
			method = "Checked"
		case element.Kind == "input" && attr.Name.Local == "value":
			method = "Set"
		case element.Kind == "slider" && attr.Name.Local == "value":
			method = "Value"
		case (element.Kind == "combo" || element.Kind == "tabs") && attr.Name.Local == "selected":
			method = "Selected"
		default:
			return true, ui.ErrTemplate
		}

		binding.Value, binding.ValueMethod = expression, method

		return true, nil
	}

	return false, nil
}

func validateWidgetBinding(element ui.Element, binding Binding) error {
	if !isNewWidget(element.Kind) {
		if binding.Change != "" || binding.Value != "" {
			return ui.ErrTemplate
		}

		return nil
	}

	if element.Kind == "tab" {
		if binding.Change != "" || binding.Value != "" || binding.Enabled != "true" {
			return ui.ErrTemplate
		}

		return nil
	}

	if binding.Change == "" || binding.Value == "" || binding.Click != "" {
		return fmt.Errorf("%s requires a value and onChange: %w", element.Kind, ui.ErrTemplate)
	}

	return nil
}

func setChangeType(kind string, binding *Binding) {
	switch kind {
	case "checkbox":
		binding.ChangeType, binding.ChangeField = "bool", "Checked"
	case "input":
		binding.ChangeType, binding.ChangeField = "string", "Text"
	case "slider":
		binding.ChangeType, binding.ChangeField = "int32", "Value"
	case "combo", "tabs":
		binding.ChangeType, binding.ChangeField = "uint32", "Selected"
	}
}
