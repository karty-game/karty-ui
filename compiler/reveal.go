package uicompiler

import (
	"encoding/xml"
	"fmt"
	"go/constant"
	"go/token"
	"go/types"
	"strconv"

	ui "github.com/karty-game/karty-ui/schema"
)

func revealAttribute(element ui.Element, binding *Binding, attr xml.Attr, expressions map[string]string) error {
	if element.Kind != "label" {
		return fmt.Errorf("reveal is only supported on labels: %w", ui.ErrTemplate)
	}

	expression, dynamic := expressions[attr.Value]
	if !dynamic {
		count, err := strconv.ParseUint(attr.Value, 10, 32)
		if err != nil || count > ui.MaxTextBytes {
			return fmt.Errorf("reveal requires a count from 0 to %d: %w", ui.MaxTextBytes, ui.ErrTemplate)
		}

		expression = strconv.FormatUint(count, 10)
	}

	// Resolve constant expressions now. Model expressions are type-checked as
	// uint32 by the generated UI.Reveal call, then bounded by the runtime.
	if evaluated, err := types.Eval(token.NewFileSet(), nil, token.NoPos, expression); err == nil && evaluated.Value != nil {
		if evaluated.Value.Kind() != constant.Int {
			return fmt.Errorf("reveal requires an integer character count: %w", ui.ErrTemplate)
		}

		count, valid := constant.Uint64Val(evaluated.Value)
		if !valid || count > ui.MaxTextBytes {
			return fmt.Errorf("reveal requires a count from 0 to %d: %w", ui.MaxTextBytes, ui.ErrTemplate)
		}
	}

	binding.Reveal = expression

	return nil
}
