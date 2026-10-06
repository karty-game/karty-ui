package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"text/template"

	"github.com/karty-game/karty-ui/compiler"
	definition "github.com/karty-game/karty-ui/schema"
)

// UIClientFiles emits same-package client functions for model-bound screens.
// Callers stage these next to copies of authored Go; never write them into src.
func UIClientFiles(components []uicompiler.Component, module string) (map[string][]byte, error) {
	return uiClientFiles(components, module, "main")
}

// UIPackageFiles emits the persistent, importable UI package.
func UIPackageFiles(components []uicompiler.Component, module string) (map[string][]byte, error) {
	return uiClientFiles(components, module, "ui")
}

func uiClientFiles(components []uicompiler.Component, module, packageName string) (map[string][]byte, error) {
	result := map[string][]byte{}
	templates := map[bool]*template.Template{}

	for _, component := range components {
		parsed := templates[component.Composition]
		if parsed == nil {
			source := uiClientSource
			if component.Composition {
				source = uiComposedClientSource
			}

			var err error

			parsed, err = template.New(component.Source).Parse(source)
			if err != nil {
				return nil, err
			}

			templates[component.Composition] = parsed
		}

		var output bytes.Buffer

		data := struct {
			uicompiler.Component

			Engine      string
			Package     string
			BackElement uint32
		}{component, module + "/.karty/engine", packageName, definition.BackElementID}
		if err := parsed.Execute(&output, data); err != nil {
			return nil, err
		}

		formatted, err := format.Source(output.Bytes())
		if err != nil {
			return nil, fmt.Errorf("%s: generated client component: %w", component.Source, err)
		}

		result[component.Source] = formatted
	}

	return result, nil
}
