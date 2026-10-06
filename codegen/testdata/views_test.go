package engine

import "testing"

func TestGeneratedLegacySnapshot(t *testing.T) {
	clicks := 0
	title := "first"
	row := uint32(0)
	view := (Game{}).ShowLegacy(func() LegacyProps {
		return LegacyProps{
			Title: title, Click: func() { clicks++ }, Select: func(value uint32) { row = value },
			Rows: []UIRow{{Text: title}},
		}
	})
	if view.Text[1] != "first" || view.Enabled[2] || view.Lists[3][0].Text != "first" {
		t.Fatal("initial legacy snapshot missing")
	}
	view.Dispatch(UIAction{Element: 2})
	view.Dispatch(UIAction{Element: 3, Row: 7})
	if clicks != 1 || row != 7 {
		t.Fatal("legacy callbacks failed")
	}
	title = "second"
	view.Invalidate()
	if view.Text[1] != "second" || view.Lists[3][0].Text != "second" {
		t.Fatal("legacy snapshot failed to refresh")
	}
}

func TestGeneratedWidgetSnapshot(t *testing.T) {
	props := LegacyWidgetsProps{Checked: true, Name: "name", Volume: 42, Selected: 1}
	props.Check = func(value bool) { props.Checked = value }
	props.Input = func(value string) { props.Name = value }
	props.Slide = func(value int32) { props.Volume = value }
	props.Select = func(value uint32) { props.Selected = value }
	view := (Game{}).ShowLegacyWidgets(func() LegacyWidgetsProps { return props })
	if view.Text[1] != "true" || view.Text[2] != "name" || view.Text[3] != "42" || view.Text[4] != "1" {
		t.Fatal("typed snapshot missing")
	}
	view.Dispatch(UIAction{Element: 1, Checked: false})
	view.Dispatch(UIAction{Element: 2, Text: "edited"})
	view.Dispatch(UIAction{Element: 3, Value: 17})
	view.Dispatch(UIAction{Element: 4, Selected: 7})
	view.Invalidate()
	if view.Text[1] != "false" || view.Text[2] != "edited" || view.Text[3] != "17" || view.Text[4] != "7" {
		t.Fatal("typed snapshot callback missing")
	}
}
