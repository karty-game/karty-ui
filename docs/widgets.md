# KartUI widgets (SDK 0.0.9 candidate)

SDK 0.0.9 uses API 0.0.7, wire protocol 10. Widgets use presentation schema 10; explicit dimensions use schema 11.
These controls work in modal screens and interactive nonmodal HUDs. Earlier
SDKs do not support these widget bindings.

| Control    | Required bindings                                                | Callback argument    | Literal attributes |
| ---------- | ---------------------------------------------------------------- | -------------------- | ------------------ |
| `checkbox` | `checked={bool}`, `onChange={func(bool)}`                        | checked state        | literal body label |
| `input`    | `value={string}`, `onChange={func(string)}`                      | edited text          | `placeholder`      |
| `slider`   | `value={int32}`, `onChange={func(int32)}`                        | integer value        | `min`, `max`       |
| `combo`    | `rows={[]UIRow}`, `selected={uint32}`, `onChange={func(uint32)}` | stable row ID        | —                  |
| `tabs`     | `selected={uint32}`, `onChange={func(uint32)}`                   | zero-based tab index | —                  |
| `tab`      | child of `tabs`                                                  | —                    | `title`            |

All value controls accept `enabled={bool}` and `class`. Tooltips use a literal
`tooltip="Help text"` on a control, label, image or nested panel; tooltips do
not emit actions. Attribute values requiring Go expressions must use braces.
Checkbox labels, tab titles, placeholders and tooltip text are literal and
render as plain text. Use a separate bound label for changing captions.

```kui
<template>
  <panel modal="true" class="settings">
    <checkbox checked={audio} onChange={func(v bool) { audio = v }} tooltip="Enable sound">Audio</checkbox>
    <input value={name} onChange={func(v string) { name = v }} placeholder="Player name"/>
    <slider min="0" max="100" value={volume} onChange={func(v int32) { volume = v }}/>
    <combo rows={rows} selected={selected} onChange={func(v uint32) { selected = v }}/>
    <tabs selected={tab} onChange={func(v uint32) { tab = v }}>
      <tab title="General"><label>General settings</label></tab>
      <tab title="Video"><label>Video settings</label></tab>
    </tabs>
  </panel>
</template>

<script setup lang="go">
import engine "example.com/game/.karty/engine"

func setup() {
    audio := true
    name := "Player"
    volume := int32(50)
    selected := uint32(1)
    tab := uint32(0)
    rows := []engine.UIRow{
        {ID: 1, Text: "Easy", Enabled: true},
        {ID: 2, Text: "Hard", Enabled: true},
    }
}
</script>

<style>
.settings
  width: 80%
  height: 80%
  padding: 16px
  gap: 8px
  align: stretch
</style>
```

For local components, import `UIRow` from the generated engine package or use a
model field with that type; local Go expressions follow ordinary Go import and
scope rules. Both adapters (same-package client and generated UI package)
use the same typed callbacks.

The host emits `EventUIChange`. The SDK derives `UIAction.Checked`, `.Text`,
`.Value` and `.Selected` and invalidates the edited component automatically.
Update the model in `onChange`; explicit `invalidate()` is unnecessary for that
component. Invalidate another component explicitly when its model changes.
Projection accepts or restores the model value even if a callback declines an
edit. Continuous edits to one control coalesce within an input batch. Idle
screens do not project every frame. `onClick` remains the button/list contract.

`combo` is a selectable dropdown backed by EbitenUI `ListComboButton`. Rows keep
caller order and stable, nonzero IDs; IDs cannot be recycled within an instance.
Zero means no selected row. A nonzero selection must exist after the complete
command batch, so selecting a row and adding it in the same batch is valid.
Disabled rows remain available for displaying the selected label but are omitted
from selectable dropdown choices. A removed selected row must be cleared or
replaced in the same batch. Dropdown height is bounded and scrollable.

`tabs` contains one or more `tab` elements, and a `tab` contains controls, nested
panels or component slots. All children remain mounted; only the selected tab
accepts input. Indices follow authored tab order, including conditionally hidden
tabs. Select a visible tab before hiding the current tab in the same projection.
A disabled tabs control keeps its selected content and rejects changes.

Slider bounds default to 0 and 100, must satisfy `min < max`, and stay within
[-1000000, 1000000]. Values must lie within those bounds. Checkbox values have
exact canonical encodings `true` and `false`; selections and slider integers use
canonical decimal encodings. Input, labels, titles, placeholders and tooltips
are limited to 4096 UTF-8 bytes. Existing limits remain: 64 elements per template,
15 element levels, 256 rows per list/combo, and 64 KiB template/command batches.
The whole batch is rejected when its final control state is invalid.

Controls use the existing font, color, background, hover/pressed/disabled,
size, margin and flex-growth styles. They do not support button icons, custom
focus styles or transitions. Tabs accept sizing, margin, flex-growth and overlay
positioning; tab contents use panel layout/background styles without scrolling
or transitions. Unsupported authoring styles warn and are ignored; serialized assets retain strict
validation. Keyboard arrows edit text,
sliders and combo choices without moving focus; Tab changes focus. Focused text
inputs temporarily capture game input even in a nonmodal HUD; clicking outside
the input or moving focus to another control restores normal game input. Native and
browser hosts use the same widget adapter. Focus and caret/selection survive
unrelated updates when the input style and scale are unchanged.
