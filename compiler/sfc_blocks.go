package uicompiler

import (
	"fmt"
	"strings"

	ui "github.com/karty-game/karty-ui/schema"
)

func sfcOpeningEnd(text string) int {
	if !strings.HasPrefix(text, "<") {
		return -1
	}

	var quote byte

	for index := 1; index < len(text); index++ {
		char := text[index]
		if quote != 0 {
			if char == quote {
				quote = 0
			}

			continue
		}

		switch char {
		case '\'', '"':
			quote = char
		case '>':
			return index
		}
	}

	return -1
}

func sfcOpening(opening string) (string, map[string]string, error) {
	text := strings.TrimSpace(opening[1 : len(opening)-1])
	nameEnd := strings.IndexAny(text, " \t\r\n")

	name := text
	if nameEnd >= 0 {
		name = text[:nameEnd]
		text = text[nameEnd:]
	} else {
		text = ""
	}

	attributes, err := sfcAttributes(text)
	if err != nil {
		return "", nil, err
	}

	valid := false

	switch name {
	case "template":
		valid = len(attributes) == 0
	case "script":
		valid = len(attributes) == 2 && attributes["setup"] == "" && attributes["lang"] == "go"
		if _, exists := attributes["setup"]; !exists {
			valid = false
		}
	case "style":
		valid = len(attributes) == 0 || (len(attributes) == 1 && attributes["lang"] == "sass")
	}

	if !valid {
		return "", nil, fmt.Errorf("unsupported SFC block %s: %w", opening, ui.ErrTemplate)
	}

	return name, attributes, nil
}

func sfcAttributes(text string) (map[string]string, error) {
	attributes := map[string]string{}

	for strings.TrimSpace(text) != "" {
		text = strings.TrimLeft(text, " \t\r\n")

		end := strings.IndexAny(text, "= \t\r\n")
		if end < 0 {
			end = len(text)
		}

		name := text[:end]
		if name != "lang" && name != "setup" {
			return nil, fmt.Errorf("unsupported SFC attribute %q: %w", name, ui.ErrTemplate)
		}

		if _, exists := attributes[name]; exists {
			return nil, fmt.Errorf("duplicate SFC attribute %s: %w", name, ui.ErrTemplate)
		}

		text = strings.TrimLeft(text[end:], " \t\r\n")

		value, remaining, err := sfcAttributeValue(name, text)
		if err != nil {
			return nil, err
		}

		text = remaining

		attributes[name] = value
	}

	return attributes, nil
}

func sfcAttributeValue(name, text string) (string, string, error) {
	if !strings.HasPrefix(text, "=") {
		if name != "setup" {
			return "", "", fmt.Errorf("SFC attribute %s requires a value: %w", name, ui.ErrTemplate)
		}

		return "", text, nil
	}

	if name == "setup" {
		return "", "", fmt.Errorf("setup is a boolean attribute: %w", ui.ErrTemplate)
	}

	text = strings.TrimLeft(text[1:], " \t\r\n")
	if len(text) == 0 || (text[0] != '\'' && text[0] != '"') {
		return "", "", fmt.Errorf("quote SFC attribute %s: %w", name, ui.ErrTemplate)
	}

	end := strings.IndexByte(text[1:], text[0])
	if end < 0 {
		return "", "", ui.ErrTemplate
	}

	value := text[1 : end+1]

	text = text[end+2:]
	if text != "" && !strings.ContainsRune(" \t\r\n", rune(text[0])) {
		return "", "", fmt.Errorf("separate SFC attributes with whitespace: %w", ui.ErrTemplate)
	}

	return value, text, nil
}

func sfcClosing(text, name string) (int, int) {
	prefix := "</" + name

	start := 0
	for start < len(text) {
		offset := strings.Index(text[start:], prefix)
		if offset < 0 {
			break
		}

		offset += start
		tail := text[offset+len(prefix):]

		trimmed := strings.TrimLeft(tail, " \t\r\n")
		if strings.HasPrefix(trimmed, ">") {
			return offset, offset + len(prefix) + len(tail) - len(trimmed) + 1
		}

		start = offset + len(prefix)
	}

	return -1, -1
}
