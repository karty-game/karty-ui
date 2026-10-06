// Package engine is a test fixture for the public adapter contract. It models
// binding callbacks only; it does not execute WASM or render graphics.
package engine

import "strconv"

type UIElement uint32
type UIAction struct {
	Element  UIElement
	Row      uint32
	Checked  bool
	Text     string
	Value    int32
	Selected uint32
}
type UIRow struct{ Text string }
type Game struct{}
type UI struct {
	Text     map[uint32]string
	Enabled  map[uint32]bool
	Visible  map[uint32]bool
	Lists    map[uint32][]UIRow
	Children map[uint32]*UI
	project  func(*UI)
	action   func(UIAction)
	props    func(any)
}

func (Game) ShowUI(_ string, project func(*UI), action func(UIAction)) *UI {
	view := &UI{
		Text: map[uint32]string{}, Enabled: map[uint32]bool{}, Visible: map[uint32]bool{},
		Lists: map[uint32][]UIRow{}, Children: map[uint32]*UI{},
	}
	view.Bind(project, action, nil)

	return view
}

func (view *UI) Bind(project func(*UI), action func(UIAction), props func(any)) {
	view.project, view.action, view.props = project, action, props
	view.Invalidate()
}

func (view *UI) Invalidate() {
	if view.project != nil {
		view.project(view)
	}
}

func (view *UI) Set(id uint32, text string, enabled bool) {
	view.Text[id], view.Enabled[id] = text, enabled
}

func (view *UI) SetVisible(id uint32, visible bool) { view.Visible[id] = visible }
func (view *UI) Rows(id uint32, rows []UIRow)       { view.Lists[id] = rows }
func (view *UI) Dispatch(action UIAction)           { view.action(action) }

func (view *UI) Child(slot uint32, _ any, asset string) (*UI, bool) {
	if child := view.Children[slot]; child != nil {
		return child, false
	}

	child := (Game{}).ShowUI(asset, nil, nil)
	view.Children[slot] = child

	return child, true
}

func (view *UI) SetProps(value any) {
	view.props(value)
	view.Invalidate()
}

func (view *UI) Checked(id uint32, value, enabled bool) {
	view.Set(id, strconv.FormatBool(value), enabled)
}
func (view *UI) Value(id uint32, value int32, enabled bool) {
	view.Set(id, strconv.FormatInt(int64(value), 10), enabled)
}
func (view *UI) Selected(id uint32, value uint32, enabled bool) {
	view.Set(id, strconv.FormatUint(uint64(value), 10), enabled)
}
