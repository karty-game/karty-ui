package uicompiler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/karty-game/karty-ui/schema"
)

// Load confines source resolution to the project, including symlinks.
func Load(directory string, entries []Source) ([]Component, error) {
	return LoadWithTheme(directory, entries, "")
}

func LoadWithTheme(directory string, entries []Source, themeSource string) ([]Component, error) {
	return LoadProjectWithTheme(directory, entries, nil, themeSource)
}

// LoadProjectWithTheme compiles runtime components after loading build-time layouts.
func LoadProjectWithTheme(
	directory string,
	entries []Source,
	layoutEntries []LayoutSource,
	themeSource string,
) ([]Component, error) {
	var components []Component

	theme := DefaultTheme()

	if themeSource != "" {
		data, err := ReadSource(directory, themeSource)
		if err != nil {
			return nil, err
		}

		theme, err = ParseTheme(themeSource, data)
		if err != nil {
			return nil, err
		}
	}

	names := map[string]bool{}

	layouts := make(map[string]Layout, len(layoutEntries))
	for _, entry := range layoutEntries {
		data, err := ReadSource(directory, entry.Source)
		if err != nil {
			return nil, err
		}

		layout, err := parseLayout(entry.Source, data)
		if err != nil {
			return nil, err
		}

		if _, duplicate := layouts[layout.Name]; duplicate {
			return nil, fmt.Errorf("%s: duplicate layout %s: %w", entry.Source, layout.Name, ui.ErrTemplate)
		}

		layouts[layout.Name] = layout
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Source, ".ui") && !strings.HasSuffix(entry.Source, ".kui") {
			continue
		}

		data, err := ReadSource(directory, entry.Source)
		if err != nil {
			return nil, err
		}

		component, err := compileWithLayouts(entry.Source, data, theme, layouts)
		if err != nil {
			return nil, err
		}

		if names[component.Name] || component.Name == "UI" || component.Name == "LevelUI" {
			return nil, fmt.Errorf("%s: duplicate or reserved component %s: %w", entry.Source, component.Name, ui.ErrTemplate)
		}

		names[component.Name] = true
		component.Asset = entry.Name
		components = append(components, component)
	}

	if err := resolveChildren(components); err != nil {
		return nil, err
	}

	return components, nil
}

func ReadSource(directory, source string) ([]byte, error) {
	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, err
	}

	if filepath.IsAbs(source) {
		return nil, ui.ErrTemplate
	}

	path, err := filepath.EvalSymlinks(filepath.Join(root, source))
	if err != nil {
		return nil, err
	}

	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, ui.ErrTemplate
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Size() > ui.MaxAssetBytes {
		return nil, ui.ErrTemplate
	}

	return os.ReadFile(path)
}

// Source identifies a named UI document without depending on a build tool.
type Source struct{ Name, Source string }

// LayoutSource identifies a build-time layout document.
type LayoutSource struct{ Source string }
