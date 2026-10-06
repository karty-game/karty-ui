# KartUI

Public KartUI language compiler, compiled schema, Karty Go adapter and VS Code
editor extension. No engine checkout or private module is required.

```sh
mise install
mise run fmt
mise run check-fmt
mise run lint
mise run test
mise run build
mise run install-editor
mise run check-editor
mise run package-editor
mise run bench
```

The shared checks in `hk.pkl` use golangci-lint for Go (including `govet`),
yamllint for YAML, Taplo for TOML, and Prettier for YAML layout, JSON, Markdown
and web files. Taplo validates syntax without downloading schemas.

`mise install` installs the pinned tools and repository pre-commit hook through
mise; use `mise run install-hooks` to reinstall it. The hook checks staged files
and applies formatting fixes while preserving unstaged work. `mise run fmt`
applies all formatters through hk without staging; `mise run check-fmt` checks
formatting without writing files. `mise run lint` runs all format and lint
checks. `mise run check` also runs tests, build and editor tests.

Generated Go and `*.generated.*` snapshots, derived output and local contributor
directories are excluded from formatting; update their owning generators instead.

- `compiler`: markup, composition, styles, themes and layout compilation.
- `schema`: bounded serialized UI definitions shared with host implementations.
- `codegen`: project-specific Go emission for the Karty adapter.
  `ProjectConfigFile` emits `.karty/config/project.go` from CLI-validated
  resolution and camera defaults using the tracked
  `codegen/templates/project-config.go.tmpl`. The CLI owns manifest parsing,
  bounds and degrees-to-radians conversion. Resolution constants are uint32;
  camera projection constants are float32. No engine source is required.
- `editors/vscode`: grammar, snippets, local completion/hover/definition providers,
  compiler-validated property catalog, standalone fixtures and VSIX packaging.

Start with [KartUI](0.0.1/kartui.md), [styles](0.0.1/styles.md)
and [themes](0.0.1/theme.md). [Repository boundaries](repository-split.md)
explain SDK pinning and the private renderer. This extraction does not promise
a generic renderer backend. Editor versions may advance independently; new
runtime capabilities still require compatible engine SDK support.

Adapter regression tests generate both same-package clients and importable UI
packages, then compile and execute their binding callbacks against a local
public-contract fixture. They cover SFC callbacks, value widgets and retained child prop
updates without an engine checkout. This is Go adapter execution, not WASM
execution or browser graphics validation.

`mise run bench` measures compiler layout rejection and adapter generation with
allocation counts. Layout benchmarks include repeated references that would
otherwise expand beyond the schema's per-template bounds.

Editor feature logic is in `editors/vscode/src/model.js`; VS Code API registration
is in `src/providers.js`. The handwritten `src/catalog.json` lists authoring
properties, value examples, target widgets, aliases and attribute help. Compiler
coverage checks every property/value/target against the actual style parser and
schema. Update that catalog when changing styles. Node tests cover incomplete
buffers, contexts, edit ranges, local navigation, cancellation and cache
invalidation; they do not constitute a manual VS Code extension-host session.
See [editor features v1](editor-features-v1.md) for current limits and next steps.
