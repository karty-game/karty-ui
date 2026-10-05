# KartUI for VS Code

Local syntax highlighting for `.kui` and `.kui.tmpl` files: KartUI components,
layouts, slot/fragment markup, CSS-like style rules, pseudo-states, theme
references, comments and strings. It recognizes the experimental SFC blocks
`<script setup lang="go">`, `<template>` and `<style>`, with Go token scopes
inside the script block. Includes bracket pairs, indentation-based folding and
component, layout, style, button and label snippets. Nested expression braces
and quoted braces are handled separately. This is a small self-contained
grammar, not the complete Go or CSS language grammar.

The extension includes a KartUI logo and an optional **KartUI** file icon theme
for `.kui` and `.kui.tmpl` files. The file theme is available through
**Preferences: File Icon Theme**; VS Code keeps the user's existing icon theme
until they select it.

No runtime extension code, network requests, telemetry, or language server.
No autocomplete/type diagnostics, formatter, tag auto-closing or Go navigation
is provided yet. In particular, Go token scopes in an SFC script do not route
requests to gopls. Highlighting does not imply compiler support for a construct.
The extension claims `.kui` to avoid taking over `.ui`, which Qt and other
tools also use. The compiler still accepts legacy `.ui` sources, but the
extension does not auto-associate that shared suffix.

For development, open this directory in VS Code and launch an Extension
Development Host with `code --extensionDevelopmentPath=/absolute/path/to/editors/vscode`.
Select **KartUI** in the language picker for a legacy `.ui` file when needed.
New KartUI documents use `.kui`; `.ui` is shared with other tools.

Run checks from the repository root:

```sh
mise exec -- npm ci --prefix editors/vscode
mise run check-editor
```

The local VSIX is built by `mise run package-editor`. Install it through
**Extensions: Install from VSIX…**, or `code --install-extension PATH_TO_VSIX`.
Reload the window after installing/updating if an already-open `.kui` file does
not switch highlighting. No Marketplace publication is required.
