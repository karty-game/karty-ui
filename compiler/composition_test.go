package uicompiler

import (
	"strings"
	"testing"
)

func TestKeyedChildSyntax(t *testing.T) {
	t.Parallel()

	source := `<template>
<panel modal="true">
 for _, item := range props.Items {
  <ItemRow key={item.ID} props={item} />
 }
 </panel>
</template>

<script setup lang="go">
func setup(props Props) {}
</script>`

	component, err := Compile("inventory.kui", []byte(source))
	if err != nil || !component.Composition || component.Template.Version != 2 || len(component.Bindings) != 1 ||
		component.Bindings[0].Child != "ItemRow" {
		t.Fatalf("child: %+v %v", component, err)
	}

	if _, err := Compile("bad.kui", []byte(strings.Replace(source, "key={item.ID}", "", 1))); err == nil {
		t.Fatal("loop without key")
	}

	if err := resolveChildren([]Component{component}); err == nil {
		t.Fatal("unknown child")
	}
}

func TestChildResolutionAndCycles(t *testing.T) {
	t.Parallel()

	components := []Component{
		{Name: "Parent", Bindings: []Binding{{Child: "Child"}}},
		{Name: "Child", Parameters: []Parameter{{Name: "props", Type: "Props"}}},
	}
	if err := resolveChildren(components); err != nil || !components[1].ChildFactory {
		t.Fatal("child factory", err)
	}

	components[0].Parameters = []Parameter{{Name: "props", Type: "Props"}}

	components[1].Bindings = []Binding{{Child: "Parent"}}
	if err := resolveChildren(components); err == nil {
		t.Fatal("recursive composition")
	}
}

func TestConditionalMarkup(t *testing.T) {
	t.Parallel()

	source := `<template>
<panel modal="true">
 if props.Empty {
  <label>Empty</label>
 } else {
  <button onClick={props.Add}>Add</button>
 }
 </panel>
</template>

<script setup lang="go">
func setup(props Props) {}
</script>`

	component, err := Compile("inventory.kui", []byte(source))
	if err != nil || !component.Conditional || len(component.Bindings) != 2 {
		t.Fatalf("conditional: %+v %v", component, err)
	}

	if component.Bindings[0].Visible != "props.Empty" || component.Bindings[1].Visible != "!(props.Empty)" {
		t.Fatalf("conditions = %#v", component.Bindings)
	}
}

func TestMountedChildSource(t *testing.T) {
	t.Parallel()

	source := `<template><panel><Picture source={props.Source} props={props.Name}/></panel></template>
<script setup lang="go">func setup(props Props) {}</script>`

	parent, err := Compile("parent.kui", []byte(source))
	if err != nil || parent.Bindings[0].Source != "props.Source" {
		t.Fatalf("source: %+v %v", parent, err)
	}

	components := []Component{parent, {Name: "Picture", Parameters: []Parameter{{Name: "name", Type: "string"}}}}
	if err := resolveChildren(components); err != nil || !components[1].SourceFactory {
		t.Fatal("source adapter missing", err)
	}

	for _, bad := range []string{`source="pack"`, `source={a} source={b}`, `unknown={a}`} {
		if _, err := Compile("bad.kui", []byte(strings.Replace(source, "source={props.Source}", bad, 1))); err == nil {
			t.Fatal("accepted", bad)
		}
	}
}
