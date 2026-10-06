package uicompiler

import (
	"testing"

	ui "github.com/karty-game/karty-ui/schema"
)

func TestWidgetBindings(t *testing.T) {
	t.Parallel()

	component, err := Compile(
		"controls.ui",
		[]byte(
			`kartui Controls(Checked bool, Name string, Volume int32, Selected uint32, Rows []UIRow, Check func(bool), Input func(string), Slide func(int32), Select func(uint32)) {
<panel modal="true">
<checkbox checked={Checked} onChange={Check} tooltip="Enable audio">Audio</checkbox>
<input value={Name} onChange={Input} placeholder="Player name"/>
<slider min="-10" max="90" value={Volume} onChange={Slide}/>
<combo rows={Rows} selected={Selected} onChange={Select}/>
<tabs selected={Selected} onChange={Select}><tab title="General"><label>Settings</label></tab><tab title="Video"><button onClick={func(){}}>Apply</button></tab></tabs>
</panel> }`,
		),
	)
	if err != nil {
		t.Fatal(err)
	}

	if component.Template.Version != ui.SchemaWidgets || len(component.Template.Elements) != 9 {
		t.Fatalf("template: %+v", component.Template)
	}

	for index, method := range []string{"Checked", "Set", "Value", "Selected", "Selected"} {
		binding := component.Bindings[index]
		if binding.ValueMethod != method || binding.Change == "" {
			t.Fatalf("binding %d: %+v", index, binding)
		}
	}

	if component.Template.Elements[0].Text != "Audio" || component.Template.Elements[0].Tooltip != "Enable audio" {
		t.Fatal("checkbox label/tooltip missing")
	}

	if component.Template.Elements[2].Value != "-10" || component.Template.Elements[2].Max != 90 {
		t.Fatal("slider bounds missing")
	}

	if component.Template.Elements[6].Parent != 6 {
		t.Fatal("tab contents lost parent")
	}
}

func TestRejectInvalidWidgets(t *testing.T) {
	t.Parallel()

	for _, markup := range []string{
		`<checkbox onChange={func(bool){}}>Missing checked</checkbox>`,
		`<checkbox checked={true} onChange={func(bool){}}>{"Dynamic label"}</checkbox>`,
		`<input value={""} onClick={func(){}}/>`,
		`<input value={""} onChange={func(string){}} placeholder={"dynamic"}/>`,
		`<slider min="5" max="5" value={int32(5)} onChange={func(int32){}}/>`,
		`<slider min="-1000001" value={int32(0)} onChange={func(int32){}}/>`,
		`<combo selected={uint32(0)} onChange={func(uint32){}}/>`,
		`<tabs selected={uint32(0)} onChange={func(uint32){}}/>`,
		`<tabs selected={uint32(0)} onChange={func(uint32){}}><label>Wrong child</label></tabs>`,
		`<tab title="Orphan"><label>Wrong parent</label></tab>`,
		`<button onClick={func(){}} tooltip={"dynamic"}>Apply</button>`,
		`<label onChange={func(string){}}>Wrong callback</label>`,
	} {
		t.Run(markup, func(t *testing.T) {
			t.Parallel()

			if _, err := Compile("bad.ui", []byte("kartui Bad() { <panel modal=\"true\">"+markup+"</panel> }")); err == nil {
				t.Fatal("accepted invalid widget")
			}
		})
	}
}
