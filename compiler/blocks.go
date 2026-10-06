package uicompiler

import (
	"fmt"
	"go/scanner"
	"go/token"

	"github.com/karty-game/karty-ui/schema"
)

// expressionEnd finds the matching brace without treating quoted/commented
// braces as delimiters. The input begins at an opening brace.
func expressionEnd(text string) (int, error) {
	file := token.NewFileSet().AddFile("Go block", -1, len(text))

	var lexer scanner.Scanner
	lexer.Init(file, []byte(text), nil, 0)

	depth := 0

	for {
		position, kind, _ := lexer.Scan()
		if kind == token.EOF {
			return 0, fmt.Errorf("unclosed Go block: %w", ui.ErrTemplate)
		}

		if kind == token.LBRACE {
			depth++
		}

		if kind == token.RBRACE {
			depth--
			if depth == 0 {
				return file.Offset(position), nil
			}
		}
	}
}
