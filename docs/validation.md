# Release validation

Validated locally on 2026-10-18 for the mounted child source addition and VS Code
extension 0.1.22. The checkout advanced externally to `ac619a0` during validation;
that commit contains the checked implementation. Existing changes were preserved.
No commit, tag, push or publication was performed by this validation session.

Commands ran from the repository root using pinned mise tools. Go workflows used
the warmed writable `GOCACHE=/tmp/karty-engine-go-cache` and
`GOLANGCI_LINT_CACHE=/tmp/karty-engine-lint-cache`, with `GOWORK=off`.

| Check                                              | Outcome                                                                                                                                                                                            |
| -------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `mise run install-editor`                          | Passed; clean lockfile install of the two pinned development dependencies, with scripts disabled.                                                                                                  |
| `mise run check`                                   | Passed formatting, all linters (zero Go issues), compiler/schema/codegen tests, Go build and all three editor test files.                                                                          |
| `GOPROXY=off mise run check`                       | Passed the same checks without sibling workspaces or module downloads, using cached public dependencies.                                                                                           |
| `mise run package-editor`                          | Sandbox attempt failed with `spawnSync zip EPERM`; identical task passed with required access. Produced `dist/editor/kartui-0.1.22.vsix`.                                                          |
| VSIX inspection using Python `zipfile`             | CRC valid; all 12 packaged source files and the license match the checkout; package and VSIX manifest versions both equal 0.1.22. No development dependencies or local agent directories included. |
| `git diff --check` and `git diff --cached --check` | Passed.                                                                                                                                                                                            |

VSIX SHA-256:
`2c4be0f1c9eafed79ef00b9b2af54dcf0ce8e73855e886142f681bf1315d9a6b`.

Go adapter tests compile and execute generated same-package and importable UI
clients against a public-contract fixture, including mounted source selection
and retained prop updates. Editor tests exercise grammar, completion/hover and
provider behavior through Node and a VS Code API harness.

These are local module, adapter and packaging checks. Actual SDK/WASM execution,
browser graphics, manual VS Code installation, hosted GitHub Actions and
GoReleaser publication were not run. Offline success uses warmed public
dependencies; it does not establish a fresh network installation. The optional
`source` attribute requires the matching SDK 0.0.13 / API 0.0.11 candidate APIs;
their public availability is not established here. See the
[versioned decision](mounted-child-sources-v1.md) for compatibility limits.
