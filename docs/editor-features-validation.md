# KartUI editor features validation

Scope: the first increment for review item 3, described in
[editor features v1](editor-features-v1.md). All checks used pinned mise tools
from the KartUI repository root, with `GOCACHE=/tmp/kartui-go-cache` and
`GOLANGCI_LINT_CACHE=/tmp/kartui-lint-cache` where applicable.

| Exact check                             | Outcome                                                                                                                    |
| --------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `mise run check`                        | Compiler/schema/codegen tests, Go build and all editor test files passed                                                   |
| `GOWORK=off GOPROXY=off mise run check` | Standalone public module tests, build and editor checks passed                                                             |
| `mise run lint`                         | Zero issues                                                                                                                |
| `mise run check-editor`                 | Final grammar, feature and provider regressions passed                                                                     |
| `mise run package-editor`               | Built `dist/editor/kartui-0.1.20.vsix`                                                                                     |
| VSIX inspection                         | Packaged manifest, README, grammar, configuration, snippets and every runtime source/catalog file match their source bytes |
| `git diff --check`                      | No whitespace errors                                                                                                       |

The compiler regression validates the complete editor style catalog, all four
compact aliases, value examples and target allowlists against the actual compiler
and schema. Feature tests cover widget attributes and typed callbacks, duplicate
attributes on both sides of the cursor, closing tags, CSS/Sass values and states,
local class/variable references, partial edits, CRLF and UTF-16 replacement ranges.
They also check suppression in scripts, nested callbacks, raw strings and comments,
including closing-block lookalikes. Large blank/deep buffers exercise resource
bounds. Provider API tests check registrations, edit/snippet conversion, safe hover
text, cancellation, document-version caching and disposal.

An early test run exposed a region-boundary error and suggestions at the end of
line comments; both were corrected. Definition tests now query complete reference
names while completion tests continue to exercise partial names. The final checks
passed. Packaging invokes zip in temporary staging with approved sandbox
escalation.

Provider tests use a VS Code API harness. No actual Extension Development Host,
manual editor interaction, extension installation or browser extension session
was run. No runtime/WASM/graphics behavior changed, so engine and CLI workflows
were not rerun for this editor-only increment. Current compiler validation remains
authoritative; Problems-panel diagnostics, formatting, project-wide symbols and
Go/gopls integration remain future work. Nothing was committed or published;
unrelated existing changes were preserved.
