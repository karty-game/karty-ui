# SFC-only authoring validation

The compiler now reads SFCs directly, parses the Go setup once, infers names from
`.kui` filenames, and generates the same typed adapters for every component.
Legacy declarations and engine view generation are removed. Authored styles use
indentation; brace/semicolon/tab blocks warn and are ignored. Declaration warnings,
original source locations, asset/expansion bounds and strict serialized validation
retain coverage on migrated fixtures. Scriptless adapter compilation/execution is
covered alongside retained props and typed widget callbacks.

From this repository root, using `GOCACHE=/tmp/kartui-go-cache`:

- `GOWORK=off GOPROXY=off mise run check`: passed Go tests, standalone build and
  editor tests, without private engine source.
- `mise run lint`: passed with no issues.
- `mise run package-editor`: produced `dist/editor/kartui-0.1.21.vsix`; grammar,
  completion and snippets now teach only SFCs with indented styles.
- A source scan found no script-before-template examples or legacy declarations
  in current samples and reference examples. Compiler block-order editing tests
  still exercise permissive order intentionally.

CLI candidate tests, sample CPU checks, engine tests/builds and the TinyGo/browser
starter smoke passed. Desktop and high-DPI touch execution used the actual web
host. See the CLI `docs/ui-sfc-validation.md` for exact commands and the standalone
CLI module-cache limitation. No manual editor-host review was performed.
Released SDK records and their historical references/templates were preserved;
the next candidate pins migrated template version 0.0.2. No commit or publication
was performed.
