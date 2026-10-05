package codegen

import (
	"bytes"
	"fmt"
	"go/format"
	"text/template"
)

// ProjectConfiguration provides validated build-time project camera defaults.
// The CLI owns manifest parsing and converts authored FOV degrees to radians.
type ProjectConfiguration struct {
	ResolutionWidth, ResolutionHeight                    uint32
	CameraFOVY, CameraNear, CameraFar, CameraOrthoHeight float32
}

// ProjectConfigFile emits the inspectable .karty/config/project.go adapter.
func ProjectConfigFile(config ProjectConfiguration) ([]byte, error) {
	parsed, err := template.New("config/project.go").Parse(projectConfigSource)
	if err != nil {
		return nil, fmt.Errorf("parse project configuration adapter: %w", err)
	}

	var output bytes.Buffer
	if err := parsed.Execute(&output, config); err != nil {
		return nil, fmt.Errorf("render project configuration adapter: %w", err)
	}

	formatted, err := format.Source(output.Bytes())
	if err != nil {
		return nil, fmt.Errorf("format project configuration adapter: %w", err)
	}

	return formatted, nil
}
