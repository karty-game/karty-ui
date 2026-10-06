# Contributing to KartUI

Contributions to the UI compiler, schema, Go adapters, documentation, and VS Code extension are welcome. Small bug fixes,
documentation improvements, and reproducible bug reports are good starting points.
For a substantial feature or a compatibility change, open an issue first so we
can agree on scope before implementation.

## Local setup

Fork and clone this repository, install [mise](https://mise.jdx.dev/), then run
these commands from the repository root:

```sh
mise install
mise run fmt
mise run check-fmt
mise run test
mise run build
mise run lint
```

`mise install` also installs the repository hk pre-commit hook. Formatting and
lint use the same pinned hk configuration locally and in CI; see the
[development guide](docs/development.md) for tool ownership and exclusions.

No private engine checkout is required. For editor changes, also run:

```sh
mise run install-editor
mise run check-editor
mise run package-editor
```

## Source ownership

`compiler/` owns parsing and compilation; `schema/` owns compiled definitions;
`codegen/` owns handwritten Go generators and templates; `editors/vscode/` owns
the extension. Keep generators and templates in Git; generated game `.karty/`
files and packaged `dist/` output do not belong in a pull request.

Update the language reference under `docs/0.0.1/` and relevant coverage tests
when changing public syntax or supported behavior. Changes requiring new runtime
support must coordinate an SDK version; editor releases alone cannot enable them.
See [development](docs/development.md) for more detail.

## Sending a pull request

Keep each pull request focused on one problem. Explain the user-visible result,
include a reproduction for a bug where possible, and list your validation results.
Add regression coverage when it protects changed behavior. Run `mise run fmt`
for Go changes and review the result before submitting. Preserve unrelated work.

Dependabot proposes weekly Go module and GitHub Actions updates, plus npm updates. Minor and patch
updates are grouped; major updates stay separate. Changes to pinned tools in
`mise.toml` and SDK-selected toolchains are reviewed manually.

## License

Contributions are made under this repository's [MIT license](LICENSE.md).
Only submit material you have the right to contribute, and retain existing
third-party copyright and license notices.
