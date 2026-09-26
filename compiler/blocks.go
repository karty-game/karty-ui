package uicompiler

import (
	"fmt"
	"go/scanner"
	"go/token"
	"strings"

	"github.com/karty-game/karty-ui/schema"
)

// separateSetup extracts the optional final top-level block before kartui.
// Scan Go tokens so comments, strings, and struct fields named setup are safe.
func (component *Component) separateSetup(prefix string) (string, error) {
	file := token.NewFileSet().AddFile(component.Source, -1, len(prefix))

	var lexer scanner.Scanner
	lexer.Init(file, []byte(prefix), nil, 0)

	depth := 0

	for {
		position, kind, literal := lexer.Scan()
		if kind == token.EOF {
			return prefix, nil
		}

		if depth == 0 && kind == token.IDENT && literal == "setup" {
			start, err := setupOpening(&lexer, file)
			if err != nil {
				return "", fmt.Errorf("%s: %w", component.Source, err)
			}

			component.setupSignature = strings.TrimSpace(prefix[file.Offset(position)+len("setup") : start])

			end, err := expressionEnd(prefix[start:])
			if err != nil {
				return "", fmt.Errorf("%s: %w", component.Source, err)
			}

			if strings.TrimSpace(prefix[start+end+1:]) != "" {
				return "", fmt.Errorf("%s: setup must immediately precede kartui: %w", component.Source, ui.ErrTemplate)
			}

			component.Setup = prefix[start+1 : start+end]
			component.hasSetup = true
			component.Local = true

			return prefix[:file.Offset(position)], nil
		}

		switch kind { //nolint:exhaustive // Only grouping tokens affect nesting depth.
		case token.LBRACE, token.LPAREN, token.LBRACK:
			depth++
		case token.RBRACE, token.RPAREN, token.RBRACK:
			depth--
		default:
		}
	}
}

func setupOpening(lexer *scanner.Scanner, file *token.File) (int, error) {
	depth := 0

	for {
		position, kind, _ := lexer.Scan()
		if kind == token.EOF {
			return 0, fmt.Errorf("expected setup Name(parameters) { Go statements }: %w", ui.ErrTemplate)
		}

		if kind == token.LBRACE && depth == 0 {
			return file.Offset(position), nil
		}

		switch kind { //nolint:exhaustive // Only grouping tokens affect nesting depth.
		case token.LPAREN, token.LBRACK, token.LBRACE:
			depth++
		case token.RPAREN, token.RBRACK, token.RBRACE:
			depth--
		default:
		}
	}
}

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
			return 0, fmt.Errorf("unclosed setup block: %w", ui.ErrTemplate)
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
