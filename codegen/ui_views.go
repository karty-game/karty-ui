package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"text/template"

	"github.com/karty-game/karty-ui/compiler"
	definition "github.com/karty-game/karty-ui/schema"
)

// UIViewFile emits project-specific strongly typed UI component entry points.
func UIViewFile(components []uicompiler.Component) ([]byte, error) {
	parsed, err := template.New("ui-views.go").Parse(uiViewsSource)
	if err != nil {
		return nil, err
	}

	var output bytes.Buffer

	data := struct {
		Components  []uicompiler.Component
		BackElement uint32
	}{components, definition.BackElementID}
	if err := parsed.Execute(&output, data); err != nil {
		return nil, err
	}

	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("generate UI Go: %w", err)
	}

	return formatted, nil
}

// UIClientFiles emits same-package client functions for model-bound screens.
// Callers stage these next to copies of authored Go; never write them into src.
func UIClientFiles(components []uicompiler.Component, module string) (map[string][]byte, error) {
	return uiClientFiles(components, module, "main")
}

// UIPackageFiles emits the persistent, importable SDK 0.4 UI package.
func UIPackageFiles(components []uicompiler.Component, module string) (map[string][]byte, error) {
	return uiClientFiles(components, module, "ui")
}

func uiClientFiles(components []uicompiler.Component, module, packageName string) (map[string][]byte, error) {
	result := map[string][]byte{}

	for _, component := range components {
		if !component.Local {
			continue
		}

		source := uiClientSource
		if component.Composition {
			source = uiComposedClientSource
		}

		parsed, err := template.New(component.Source).Parse(source)
		if err != nil {
			return nil, err
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
