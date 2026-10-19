package uicompiler

import (
	"testing"
)

func TestLabelRevealRetainsCompleteTextAndIndependentCount(t *testing.T) {
	t.Parallel()

	for _, attribute := range []string{`reveal="0"`, `reveal="4096"`, `reveal={uint32(7)}`, `reveal={props.Count}`} {
		component, err := Compile("line.kui", []byte(`<template><panel><label `+attribute+`>{props.Text}</label></panel></template>
<script setup lang="go">
type Props struct {Text string; Count uint32}
func setup(props Props) {}
</script>`))
		if err != nil {
			t.Fatal(attribute, err)
		}

		binding := component.Bindings[0]
		if binding.Text != "props.Text" || binding.Reveal == "" || binding.Value != "" {
			t.Fatal("reveal replaced the complete layout text or widget value", binding)
		}
	}
}

func TestInvalidLabelRevealIsRejected(t *testing.T) {
	t.Parallel()

	for _, markup := range []string{
		`<button reveal="2">Wrong control</button>`,
		`<input reveal={uint32(1)} value={""} onChange={func(string){}}/>`,
		`<label reveal="-1">Negative</label>`,
		`<label reveal="1.5">Fractional</label>`,
		`<label reveal="4097">Oversized</label>`,
		`<label reveal="none">Not a count</label>`,
		`<label reveal={-1}>Negative constant</label>`,
		`<label reveal={2.0}>Float constant</label>`,
		`<label reveal={"2"}>String constant</label>`,
		`<label reveal={1<<20}>Oversized expression</label>`,
		`<label reveal={uint32(5000)}>Oversized cast</label>`,
		`<label reveal="1" reveal="2">Duplicate</label>`,
	} {
		if _, err := Compile("bad.kui", []byte(`<template><panel>`+markup+`</panel></template>`)); err == nil {
			t.Fatal("accepted invalid reveal", markup)
		}
	}
}
