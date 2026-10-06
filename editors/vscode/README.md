# KartUI for VS Code

Completion, hover help and local style navigation, plus syntax highlighting for `.kui` and `.kui.tmpl` files: KartUI components,
layouts, slot/fragment markup, indented style rules, pseudo-states, theme
references, comments and strings. It recognizes the SFC blocks
`<script setup lang="go">`, `<template>` and `<style>`, with Go token scopes
inside the script block. Includes bracket pairs, indentation-based folding and
component, layout, style, button and label snippets. Nested expression braces
and quoted braces are handled separately. This is a small self-contained
grammar, not the complete Go or CSS language grammar.

The extension includes a KartUI logo and an optional **KartUI** file icon theme
for `.kui` and `.kui.tmpl` files. The file theme is available through
**Preferences: File Icon Theme**; VS Code keeps the user's existing icon theme
until they select it.

The local extension provides:

- Context-aware completion for markup tags, widget attributes and typed callback
  snippets, including the matching enclosing closing tags.
- Style properties, short aliases and value suggestions in indented
  styles. Hover a property to see its units, bounds and prerequisites.
- Class completion from this file's styles, and Sass variable completion from
  declarations before the cursor. Ctrl+click / Go to Definition navigates to
  these local declarations.

Use Ctrl+Space to request completion. Suggestions also appear after `<`, spaces,
`:`, `.`, `$` and attribute quotes. Existing attributes are excluded; state rules
suggest their three supported properties. Go scripts, embedded Go expressions
and comments do not receive KartUI suggestions. Unsaved buffers are supported.

This is an incremental editor service. It does not provide Go completion or type
checking, compiler diagnostics in the Problems panel, cross-file component or
layout discovery, formatting, rename, or tag auto-closing. Go token scopes do not
route requests to gopls. Custom theme tokens/images are described in hover help;
the extension does not load project themes. Local class navigation covers
unqualified classes in this file; `Frame.content` needs cross-file layout analysis.
The extension runs locally in the desktop/remote VS Code extension host and does
not provide a browser extension entry point. It makes no network requests, starts
no subprocesses and has no telemetry. It reads only the current editor buffer,
so completion also works in untitled, virtual and untrusted documents.
KartUI documents use `.kui`; template files use `.kui.tmpl`.

For development, open this directory in VS Code and launch an Extension
Development Host with `code --extensionDevelopmentPath=/absolute/path/to/editors/vscode`.

Run checks from the repository root:

```sh
mise exec -- npm ci --prefix editors/vscode
mise run check-editor
```

The local VSIX is built by `mise run package-editor`. Install it through
**Extensions: Install from VSIX…**, or `code --install-extension PATH_TO_VSIX`.
Reload the window after installing/updating if an already-open `.kui` file does
not switch highlighting. No Marketplace publication is required.

The KUI grammar highlights indented `<style>` and `<style lang="sass">` blocks,
component variables, nested `&:state` selectors, percentages and `px` values.
Use `kui`, `style-sass`, `state-sass` and `size-ui` snippets for the compact form.
Short properties `direction`, `align`, `justify` and `grow` are supported by the
compiler in indented styles. Indentation rules recognize style selectors,
nested states and media blocks.

Default `kartui` and `kui` snippets create SFC components. `layout` creates a
layout with local styles; `style-layout` inserts an explicit `Frame.content`
override. Use `checkbox`, `input`, `slider`, `combo`, `tabs`, `tooltip` and
`settings-ui` for typed controls or a complete settings screen. Style property
and responsive snippets use indentation; insert them inside an existing style
block. Component snippets put the template first. Compiler warnings use
original `file:line:column` locations. The compiler remains authoritative; editor
suggestions do not replace validation. See [editor features v1](../../docs/editor-features-v1.md)
for scope, implementation and validation.
