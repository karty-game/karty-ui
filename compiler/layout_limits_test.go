package uicompiler

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	ui "github.com/karty-game/karty-ui/schema"
)

func repeatedLayouts(tb testing.TB, depth int) map[string]Layout {
	tb.Helper()

	layouts := make(map[string]Layout, depth+1)
	for index := 0; index <= depth; index++ {
		inside := "<label>Leaf</label>"
		if index < depth {
			inside = fmt.Sprintf("<L%d/><L%d/>", index+1, index+1)
		}

		name := fmt.Sprintf("L%d", index)

		layout, err := parseLayout(name+".kui", []byte(`<template>
<panel>`+inside+`<slot/></panel>
</template>`))
		if err != nil {
			tb.Fatal(err)
		}

		layouts[name] = layout
	}

	return layouts
}

func TestRepeatedLayoutsStopAtElementLimit(t *testing.T) {
	t.Parallel()

	layouts := repeatedLayouts(t, 12)

	root, err := parseMarkupTree("<L0/>")
	if err != nil {
		t.Fatal(err)
	}

	budget := &layoutBudget{}

	var styles []styleSource

	_, err = expandNode(root, layouts, nil, map[string]bool{}, &styles, budget, 0)
	if !errors.Is(err, ui.ErrTemplate) || budget.elements != ui.MaxElements {
		t.Fatalf("unbounded expansion: count=%d err=%v", budget.elements, err)
	}
}

func TestLayoutElementAndDepthBoundaries(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("shell.kui", []byte(`<template>
<panel><slot/></panel>
</template>`))
	if err != nil {
		t.Fatal(err)
	}

	layouts := map[string]Layout{"Shell": layout}

	for _, count := range []int{ui.MaxElements, ui.MaxElements + 1} {
		source := []byte(`<template>
<Shell>` + strings.Repeat("<label/>", count) + `</Shell>
</template>`)

		_, err := compileWithLayouts("menu.kui", source, DefaultTheme(), layouts)
		if (count == ui.MaxElements) != (err == nil) {
			t.Fatalf("element boundary %d: %v", count, err)
		}
	}

	for _, panels := range []int{maxElementDepth - 1, maxElementDepth} {
		markup := strings.Repeat("<panel>", panels) + "<label/>" + strings.Repeat("</panel>", panels)

		for _, useLayout := range []bool{false, true} {
			root := "panel"
			selectedLayouts := layouts

			if useLayout {
				root = "Shell"
			} else {
				selectedLayouts = nil
			}

			source := []byte(`<template>
<` + root + `>` + markup + `</` + root + `>
</template>`)

			_, err := compileWithLayouts("menu.kui", source, DefaultTheme(), selectedLayouts)
			if (panels < maxElementDepth) != (err == nil) {
				t.Fatalf("panel depth %d, layout=%t: %v", panels, useLayout, err)
			}
		}
	}
}

func TestRepeatedSlotFillsStopDuringProjection(t *testing.T) {
	t.Parallel()

	layout, err := parseLayout("repeat.kui", []byte(`<template>
<panel>`+strings.Repeat("<slot/>", ui.MaxElements)+`</panel>
</template>`))
	if err != nil {
		t.Fatal(err)
	}

	fill, err := parseMarkupTree("<panel>" + strings.Repeat("<label/>", ui.MaxElements) + "</panel>")
	if err != nil {
		t.Fatal(err)
	}

	_, err = projectSlots(layout.root, map[string][]markupChild{"": {{Node: &fill}}})
	if !errors.Is(err, ui.ErrTemplate) {
		t.Fatalf("accepted oversized repeated slot projection: %v", err)
	}

	leaf, err := parseMarkupTree("<label/>")
	if err != nil {
		t.Fatal(err)
	}

	projected, err := projectSlots(layout.root, map[string][]markupChild{"": {{Node: &leaf}}})
	if err != nil || len(projected.Children) != ui.MaxElements {
		t.Fatalf("rejected repeated fill at the element boundary: %v", err)
	}
}

func BenchmarkRepeatedLayoutRejection(b *testing.B) {
	layouts := repeatedLayouts(b, 12)
	source := []byte(`<template>
<L0/>
</template>`)

	b.ReportAllocs()
	b.ResetTimer()

	for range b.N {
		if _, err := compileWithLayouts("menu.kui", source, DefaultTheme(), layouts); err == nil {
			b.Fatal("accepted oversized layout")
		}
	}
}
