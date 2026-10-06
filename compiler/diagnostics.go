package uicompiler

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

// SourceLocation identifies a one-based byte column in an authored source file.
type SourceLocation struct {
	Source       string
	Line, Column int
}

func (location SourceLocation) String() string {
	if location.Line > 0 {
		return fmt.Sprintf("%s:%d:%d", location.Source, location.Line, location.Column)
	}

	return location.Source
}

// Diagnostic describes an ignored authoring style. Locations refer to original
// source files, including layout files, rather than generated compiler input.
type Diagnostic struct {
	SourceLocation

	Message string
}

func (diagnostic Diagnostic) String() string {
	if diagnostic.Source == "" {
		return boundedStyleWarningText(diagnostic.Message)
	}

	return boundedStyleWarningText(diagnostic.SourceLocation.String() + ": " + diagnostic.Message)
}

// Diagnostics returns a copy of the component's bounded style warnings.
func (component *Component) Diagnostics() []Diagnostic { return slices.Clone(component.styleWarnings) }

// SourceError preserves the validation cause while identifying authored input.
// Callers can use errors.As to obtain its location and errors.Is for the cause.
type SourceError struct {
	SourceLocation

	Message string
	Cause   error
}

func (failure *SourceError) Error() string {
	return failure.String() + ": " + failure.Message + ": " + failure.Cause.Error()
}
func (failure *SourceError) Unwrap() error { return failure.Cause }

func sourceError(location SourceLocation, message string, cause error) error {
	if _, ok := errors.AsType[*SourceError](cause); ok {
		return cause
	}

	return &SourceError{SourceLocation: location, Message: message, Cause: cause}
}

func sourcePosition(source, text string, offset int) SourceLocation {
	offset = max(0, min(offset, len(text)))
	prefix := text[:offset]

	return SourceLocation{Source: source, Line: strings.Count(prefix, "\n") + 1, Column: offset - strings.LastIndexByte(prefix, '\n')}
}

type styleSpan struct {
	offset   int
	location SourceLocation
}

func sourceLineStarts(text string) []int {
	result := []int{0}

	for index, char := range text {
		if char == '\n' {
			result = append(result, index+1)
		}
	}

	return result
}

func (document styleSource) position(offset int) SourceLocation {
	if len(document.spans) > 0 {
		index := sort.Search(len(document.spans), func(index int) bool { return document.spans[index].offset > offset }) - 1

		return document.spans[max(0, index)].location
	}

	if len(document.lineStarts) == 0 {
		return sourcePosition(document.source, document.original, document.start+offset)
	}

	absolute := max(0, min(document.start+offset, len(document.original)))
	line := sort.SearchInts(document.lineStarts, absolute+1) - 1

	return SourceLocation{Source: document.source, Line: line + 1, Column: absolute - document.lineStarts[line] + 1}
}

type mappedStyle struct {
	strings.Builder

	spans []styleSpan
}

func (output *mappedStyle) write(text string, location SourceLocation) {
	output.spans = append(output.spans, styleSpan{offset: output.Len(), location: location})
	output.WriteString(text)
}
func (output *mappedStyle) append(other *mappedStyle) {
	start := output.Len()
	for _, span := range other.spans {
		span.offset += start
		output.spans = append(output.spans, span)
	}

	output.WriteString(other.String())
}

func styleLineLocation(document styleSource, line int) SourceLocation {
	offset := 0
	for range line - 1 {
		next := strings.IndexByte(document.text[offset:], '\n')
		if next < 0 {
			break
		}

		offset += next + 1
	}

	end := strings.IndexByte(document.text[offset:], '\n')
	if end < 0 {
		end = len(document.text) - offset
	}

	raw := document.text[offset : offset+end]

	return document.position(offset + len(raw) - len(strings.TrimLeft(raw, " \t")))
}

type expressionError struct {
	fragment string
	cause    error
}

func (failure *expressionError) Error() string { return failure.cause.Error() }
func (failure *expressionError) Unwrap() error { return failure.cause }
func locateExpressionFailure(document styleSource, cause error) error {
	location := document.position(0)
	if failure, ok := errors.AsType[*expressionError](cause); ok {
		if offset := strings.Index(document.text, failure.fragment); offset >= 0 {
			location = document.position(offset)
		}
	}

	return sourceError(location, "invalid Go expression", cause)
}
