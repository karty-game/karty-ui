package uicompiler

import (
	"fmt"
	"strconv"
	"strings"

	ui "github.com/karty-game/karty-ui/schema"
)

func parseStyleLength(value string) (ui.Length, error) {
	if value == "auto" {
		return ui.Length{}, nil
	}

	if percent, ok := strings.CutSuffix(value, "%"); ok {
		return parseStylePercent(percent)
	}

	pixels := strings.TrimSuffix(value, "px")
	for _, digit := range pixels {
		if digit < '0' || digit > '9' {
			return ui.Length{}, ui.ErrTemplate
		}
	}

	number, err := strconv.ParseUint(pixels, 10, 16)
	if err != nil || number > ui.MaxDimensionPixels {
		return ui.Length{}, fmt.Errorf("invalid dimension %q: %w", value, ui.ErrTemplate)
	}

	return ui.Length{Unit: ui.LengthPixels, Value: uint16(number)}, nil
}

func (component *Component) raiseSizingSchema() {
	const sizing = ui.Style2Width | ui.Style2Height
	if (component.Template.Panel.Set2|component.Template.ResponsivePanel.Set2)&sizing != 0 {
		component.Template.Version = max(component.Template.Version, ui.SchemaSizing)
	}

	for _, element := range component.Template.Elements {
		if (element.Style.Set2|element.ResponsiveStyle.Set2)&sizing != 0 {
			component.Template.Version = max(component.Template.Version, ui.SchemaSizing)
		}
	}
}

func parseStylePercent(percent string) (ui.Length, error) {
	const (
		maxPercent    = 100
		decimalPlaces = 2
	)

	whole, fraction, _ := strings.Cut(percent, ".")
	if len(fraction) > decimalPlaces || (strings.Contains(percent, ".") && fraction == "") {
		return ui.Length{}, ui.ErrTemplate
	}

	for _, digit := range whole + fraction {
		if digit < '0' || digit > '9' {
			return ui.Length{}, ui.ErrTemplate
		}
	}

	integer, err := strconv.ParseUint(whole, 10, 16)
	if err != nil || integer > maxPercent {
		return ui.Length{}, ui.ErrTemplate
	}

	decimal := uint64(0)
	if fraction != "" {
		decimal, err = strconv.ParseUint(fraction+strings.Repeat("0", decimalPlaces-len(fraction)), 10, 16)
	}

	total := integer*(ui.PercentScale/maxPercent) + decimal
	if err != nil || total > ui.PercentScale {
		return ui.Length{}, ui.ErrTemplate
	}

	return ui.Length{Unit: ui.LengthPercent, Value: uint16(total)}, nil
}
