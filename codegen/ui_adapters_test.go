package codegen

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	uicompiler "github.com/karty-game/karty-ui/compiler"
)

func writeAdapterTestFile(t *testing.T, root, name string, data []byte) {
	t.Helper()

	directory, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := directory.Close(); err != nil {
			t.Error(err)
		}
	})

	if err := directory.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := directory.WriteFile(name, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestGeneratedAdaptersCompileAndUpdateProps(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for name, source := range map[string]string{
		"static.kui": `<template><panel><label>Ready</label></panel></template>`,
		"widgets.kui": `<template>
<panel modal="true"><checkbox checked={props.Checked} onChange={props.Check}>Audio</checkbox><input value={props.Name} onChange={props.Input}/><slider value={props.Volume} onChange={props.Slide}/><combo rows={[]engine.UIRow{{Text:"Choice"}}} selected={props.Selected} onChange={props.Select}/><tabs selected={uint32(0)} onChange={props.Select}><tab title="General"><label>Settings</label></tab></tabs></panel>
</template>

<script setup lang="go">
import engine "example.com/adaptertest/.karty/engine"
type WidgetProps struct { Checked bool; Name string; Volume int32; Selected uint32; Check func(bool); Input func(string); Slide func(int32); Select func(uint32) }

func setup(props *WidgetProps) {}
</script>`,
		"widget-parent.kui": `<template>
<panel modal="true"><Widgets props={props}/></panel>
</template>

<script setup lang="go">
func setup(props *WidgetProps) {}
</script>`,

		"parent.kui": `<template>
<panel modal="true" onBack={callback}><Child props={view.Title}/><button onClick={callback}>Click</button></panel>
</template>

<script setup lang="go">
type ParentProps struct { Title string; Click func() }

func setup(view *ParentProps) {
callback := view.Click
}
</script>`,
		"child.kui": `<template><panel><label>{value}</label></panel></template>

<script setup lang="go">
func setup(value string) {}
</script>
<style>
$width: 80%
panel
  width: $width
  direction: column
label
  width: 100%
</style>`,
		"plain.kui": `<template>
<panel>
if true {
<label>{value}</label>
}
</panel>
</template>

<script setup lang="go">
func setup(value string) {}
</script>`,
	} {
		writeAdapterTestFile(t, root, name, []byte(source))
	}

	components, err := uicompiler.Load(root, []uicompiler.Source{
		{Name: "static", Source: "static.kui"},
		{Name: "parent", Source: "parent.kui"},
		{Name: "child", Source: "child.kui"},
		{Name: "plain", Source: "plain.kui"},
		{
			Name:   "widgets",
			Source: "widgets.kui",
		},
		{Name: "widget-parent", Source: "widget-parent.kui"},
	})
	if err != nil {
		t.Fatal(err)
	}

	writeAdapterTestFile(t, root, "go.mod", []byte("module example.com/adaptertest\n\ngo 1.27.0\n"))

	for source, target := range map[string]string{
		"engine.go": ".karty/engine/engine.go",
	} {
		data, err := os.ReadFile(filepath.Join("testdata", source))
		if err != nil {
			t.Fatal(err)
		}

		writeAdapterTestFile(t, root, target, data)
	}

	testSource, err := os.ReadFile(filepath.Join("testdata", "client_test.go.tmpl"))
	if err != nil {
		t.Fatal(err)
	}

	testTemplate, err := template.New("client_test.go").Parse(string(testSource))
	if err != nil {
		t.Fatal(err)
	}

	for _, adapter := range []struct {
		directory, packageName string
		generate               func([]uicompiler.Component, string) (map[string][]byte, error)
	}{
		{"client", "main", UIClientFiles}, {"ui", "ui", UIPackageFiles},
	} {
		files, err := adapter.generate(components, "example.com/adaptertest")
		if err != nil {
			t.Fatal(err)
		}

		for name, data := range files {
			writeAdapterTestFile(t, root, filepath.Join(adapter.directory, name+".go"), data)
		}

		var output bytes.Buffer
		if err := testTemplate.Execute(&output, struct{ Package string }{adapter.packageName}); err != nil {
			t.Fatal(err)
		}

		writeAdapterTestFile(t, root, filepath.Join(adapter.directory, "adapter_test.go"), output.Bytes())
	}

	command := exec.CommandContext(t.Context(), "go", "test", "./client", "./ui")
	command.Dir = root

	command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated adapter compilation/execution: %v\n%s", err, output)
	}
}

func BenchmarkUIClientFiles(b *testing.B) {
	component, err := uicompiler.Compile("menu.kui", []byte(`<template>
<panel><label>{value}</label></panel>
</template>

<script setup lang="go">
func setup(value string) {}
</script>`))
	if err != nil {
		b.Fatal(err)
	}

	components := make([]uicompiler.Component, 32)
	for index := range components {
		components[index] = component
		components[index].Source = strings.Repeat("x", index+1) + ".kui"
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := UIClientFiles(components, "example.com/game"); err != nil {
			b.Fatal(err)
		}
	}
}
