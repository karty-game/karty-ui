# KartUI for VS Code

Local syntax highlighting for `.ui` and `.ui.tmpl` files: KartUI components,
layouts, setup blocks, slot/fragment markup, Go-like parameter/expression
tokens, CSS-like style rules, pseudo-states, theme references, comments and
strings. Includes bracket pairs, indentation-based folding and component,
layout, style, button and label snippets. Nested expression braces and quoted
braces are handled separately. This is a small self-contained grammar, not the
complete Go or CSS language grammar.

No runtime extension code, network requests, telemetry, or language server.
No autocomplete/type diagnostics, formatter, tag auto-closing or Go navigation
is provided yet. Highlighting does not imply compiler support for a construct.
The repository workspace explicitly associates `.ui` and `.ui.tmpl` with
KartUI because `.ui` is also used by Qt and other extensions.

For development, open this directory in VS Code and launch an Extension
Development Host with `code --extensionDevelopmentPath=/absolute/path/to/editors/vscode`.
Select **KartUI** in the language picker if `.ui` is associated with another
language. `.ui` is a shared extension (for example Qt); association is configurable.

Run checks from the repository root:

```sh
mise exec -- npm ci --prefix editors/vscode
mise run check-editor
```

The local VSIX is built by `mise run package-editor`. Install it through
**Extensions: Install from VSIX…**, or `code --install-extension PATH_TO_VSIX`.
Reload the window after installing/updating if an already-open `.ui` file does
not switch highlighting. No Marketplace publication is required.
