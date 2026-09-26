# KartUI

Public KartUI language compiler, compiled schema, Karty Go adapter and VS Code
syntax extension. No engine checkout or private module is required.

```sh
mise install
mise run test
mise run build
mise run install-editor
mise run check-editor
mise run package-editor
```

- `compiler`: markup, composition, styles, themes and layout compilation.
- `schema`: bounded serialized UI definitions shared with host implementations.
- `codegen`: project-specific Go emission for the Karty adapter.
- `editors/vscode`: grammar, snippets, standalone fixtures and VSIX packaging.

Start with [KartUI](0.0.1/kartui.md), [styles](0.0.1/styles.md)
and [themes](0.0.1/theme.md). [Repository boundaries](repository-split.md)
explain SDK pinning and the private renderer. This extraction does not promise
a generic renderer backend. Editor versions may advance independently; new
runtime capabilities still require compatible engine SDK support.
