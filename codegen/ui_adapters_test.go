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
		"widgets.ui": `import engine "example.com/adaptertest/.karty/engine"
type WidgetProps struct { Checked bool; Name string; Volume int32; Selected uint32; Check func(bool); Input func(string); Slide func(int32); Select func(uint32) }
setup Widgets(props *WidgetProps) {}
kartui Widgets { <panel modal="true"><checkbox checked={props.Checked} onChange={props.Check}>Audio</checkbox><input value={props.Name} onChange={props.Input}/><slider value={props.Volume} onChange={props.Slide}/><combo rows={[]engine.UIRow{{Text:"Choice"}}} selected={props.Selected} onChange={props.Select}/><tabs selected={uint32(0)} onChange={props.Select}><tab title="General"><label>Settings</label></tab></tabs></panel> }`,
		"widget-parent.ui": `setup WidgetParent(props *WidgetProps) {}
kartui WidgetParent { <panel modal="true"><Widgets props={props}/></panel> }`,
		"legacy-widgets.ui": `kartui LegacyWidgets(Checked bool, Name string, Volume int32, Selected uint32, Check func(bool), Input func(string), Slide func(int32), Select func(uint32)) {
<panel modal="true"><checkbox checked={Checked} onChange={Check}>Audio</checkbox><input value={Name} onChange={Input}/><slider value={Volume} onChange={Slide}/><combo rows={[]UIRow{{Text:"Choice"}}} selected={Selected} onChange={Select}/><tabs selected={uint32(0)} onChange={Select}><tab title="General"><label>Settings</label></tab></tabs></panel> }`,
		"parent.ui": `type ParentProps struct { Title string; Click func() }
setup Parent(view *ParentProps) { callback := view.Click }
kartui Parent { <panel modal="true" onBack={callback}><Child props={view.Title}/><button onClick={callback}>Click</button></panel> }`,
		"child.kui": `<script setup lang="go">
func setup(value string) {}
</script><template><panel><label>{value}</label></panel></template>
<style>
$width: 80%
panel
  width: $width
  direction: column
label
  width: 100%
</style>`,
		"plain.ui": "setup Plain(value string) {}\nkartui Plain { <panel>\nif true {\n<label>{value}</label>\n}\n</panel> }",
		"legacy.ui": `kartui Legacy(Title string, Click func(), Select func(uint32), Rows []UIRow) {
<panel modal="true"><label>{Title}</label><button enabled={false} onClick={Click}>Click</button><list rows={Rows} onClick={Select}/></panel> }`,
	} {
		writeAdapterTestFile(t, root, name, []byte(source))
	}

	components, err := uicompiler.Load(root, []uicompiler.Source{
		{Name: "parent", Source: "parent.ui"},
		{Name: "child", Source: "child.kui"},
		{Name: "plain", Source: "plain.ui"},
		{Name: "legacy", Source: "legacy.ui"},
		{
			Name:   "widgets",
			Source: "widgets.ui",
		},
		{Name: "widget-parent", Source: "widget-parent.ui"},
		{Name: "legacy-widgets", Source: "legacy-widgets.ui"},
	})
	if err != nil {
		t.Fatal(err)
	}

	writeAdapterTestFile(t, root, "go.mod", []byte("module example.com/adaptertest\n\ngo 1.27.0\n"))

	for source, target := range map[string]string{
		"engine.go": ".karty/engine/engine.go", "views_test.go": ".karty/engine/views_test.go",
	} {
		data, err := os.ReadFile(filepath.Join("testdata", source))
		if err != nil {
			t.Fatal(err)
		}

		writeAdapterTestFile(t, root, target, data)
	}

	views, err := UIViewFile(components)
	if err != nil {
		t.Fatal(err)
	}

	writeAdapterTestFile(t, root, ".karty/engine/views.go", views)

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

	command := exec.CommandContext(t.Context(), "go", "test", "./client", "./ui", "./.karty/engine")
	command.Dir = root

	command.Env = append(os.Environ(), "GOWORK=off", "GOTOOLCHAIN=local")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated adapter compilation/execution: %v\n%s", err, output)
	}
}

func BenchmarkUIClientFiles(b *testing.B) {
	component, err := uicompiler.Compile("menu.ui", []byte(`setup Menu(value string) {}
kartui Menu { <panel><label>{value}</label></panel> }`))
	if err != nil {
		b.Fatal(err)
	}

	components := make([]uicompiler.Component, 32)
	for index := range components {
		components[index] = component
		components[index].Source = strings.Repeat("x", index+1) + ".ui"
	}

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := UIClientFiles(components, "example.com/game"); err != nil {
			b.Fatal(err)
		}
	}
}
