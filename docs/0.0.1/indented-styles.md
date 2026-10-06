# Indented styles and container-relative dimensions

These additions require the SDK 0.0.9 candidate when dimensions are used
(presentation schema 11). Existing brace styles and SDK compatibility remain
unchanged. Indentation, variables and short property names compile away; only
`width` and `height` require the new presentation schema.

A single-file `.kui` component may use `<style lang="sass">`. For less boilerplate,
plain `<style>` also accepts indented syntax when its contents have no braces.
Existing brace-based contents still use the original parser. This is a bounded,
Sass-inspired syntax implemented by KartUI, with no Sass executable dependency.

```kui
<template>
  <panel class="screen">
    <button class="primary" onClick={ func() {} }>Play</button>
  </panel>
</template>

<style>
$space: theme.spacing.gap

.screen
  width: 80%
  height: 60%
  padding: 16px
  direction: column
  gap: $space
  align: stretch

.primary
  width: 100%
  height: 44px
  background: theme.colors.primary
  &:hover
    background: theme.colors.primary-hover
</style>
```

Indent with spaces. Each sibling has the same indentation; each child is
indented further. Braces, semicolons and tabs are rejected in indented styles.
Blank lines and `//` comments are allowed. Top-level `$name: value` variables
are component-local, declared once, and may reference a literal, theme token,
or an already declared variable. A property value can reference one whole
variable. There is no interpolation, arithmetic, import, mixin or loop syntax.

Selectors are existing element names, one class, or an explicit layout-class
override such as `Frame.content`. Nest `&:hover`, `&:focus`,
`&:pressed`, or `&:disabled` under a selector for its interactive state.
Only background, background-image and color have state variants. Descendant
selectors are unsupported and warn when ignored. A top-level `@media (max-width: 480)` may contain
indented selectors and nested states; the existing single-breakpoint bounds
remain 240 through 1024 logical pixels. Layout SFCs accept the same syntax and
resolve their variables before projection into a component.

Short names work in both style syntaxes:

| Short name | Existing name |
| --- | --- |
| `direction` | `flex-direction` |
| `align` | `align-items` |
| `justify` | `justify-content` |
| `grow` | `flex-grow` |

Using both names for the same property in one rule warns and ignores the later
declaration.

`width` and `height` accept `auto`, integer logical pixels (`44px` or `44`),
or percentages from `0%` through `100%` with up to two decimal places
(`33.33%`). Pixel dimensions range from 0 through 2048. Existing spacing,
font, min/max, icon-size and inset properties also accept an optional `px`
suffix within their existing bounds. Growth weights and transition times
retain their existing integer units.

Percentages use the **immediate parent's inner size after padding**. Root
percentages use the viewport. They do not use remaining row space or the
viewport for every nested element. Margins and sibling gaps remain outside
these sizes, so two 50%-width children plus a gap can overflow. Use `grow: 1`
with automatic widths to divide remaining space equally instead.

`auto` preserves intrinsic sizing and allows stretch/growth. Explicit dimensions
win over stretch and growth on their axis; authored min/max constraints still
clamp the resulting size. A responsive `width: auto` clears an explicit base
width. Overlays with both left/right or top/bottom insets derive that axis from
the insets. Min/max remain optional safety limits, rather than the primary way
to request a size.

A root with neither `width` nor `height` keeps its existing full-viewport behavior. Intrinsic
measurement does not resolve percentage children against an unknown parent:
it uses their intrinsic content size, then resolves the percentages when the
parent has its rectangle. For predictable percentage heights, give the parent
a definite height. This avoids cyclic percentage-driven auto measurement.

The runtime uses bounded integer pixel/percentage values and does not parse
style source or resolve variables. The VS Code grammar and snippets recognize
both forms, nested states, variables, short names and dimensions.

## Ignored styles and build warnings

Unsupported properties or selectors, invalid values, unresolved tokens/variables,
and out-of-range sizes warn and are ignored. Other valid declarations still
apply. An invalid override preserves the previous valid value; without one,
the widget uses its usual default. This applies to brace and indented styles,
layout styles and responsive overrides. Shared classes apply each property only
to widgets that support it, warning for unsupported targets.

```kui
<template>
  <panel class="screen"><label>Ready</label></panel>
</template>
<style>
.screen
  width: 80%
  height: 9999px
  gap: 8px
</style>
```

This compiles with a warning for `height`; `width` and `gap` still apply.
Conflicting min/max limits ignore the declaration that creates the conflict.
Insets without `position: absolute`, icon sizing without an icon, and slide
transitions without a positive duration are ignored with warnings. Unsupported
media conditions and additional breakpoints are ignored. Invalid variable
redefinitions preserve the previous variable value. A duplicate property in
one rule preserves the first accepted declaration.

Build tools display warnings as `file:line:column` with the affected style.
Locations identify the original file, including nested layout styles.
Columns are one-based byte positions. `Component.Diagnostics()` exposes these
locations and messages as structured data. Compiler callers
can read `Component.StyleWarnings()`; it returns a copy and does not write logs.
Diagnostics are bounded to 128 unique warnings plus a truncation notice.
Malformed component structure, inconsistent indentation, excessive nesting,
source-size limits and invalid serialized assets remain compilation/validation
errors. Ignoring an authoring declaration never relaxes asset bounds.

## Layout ownership

Layouts style only markup they author, including default slot content. Projected
slot fills retain their caller's styles, and nested layouts have separate scopes.
A consuming component uses `Frame.content` to override the `Frame` layout's
`.content` class explicitly. The override applies to every instance of that
layout in the component. Nested state forms such as `Frame.primary` with
`&:hover`, and media overrides, work with the same qualification. Unqualified
classes in the consuming component do not reach into the layout. This replaces
implicit shared layout/component class matching in the unreleased candidate.
