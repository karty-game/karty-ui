# KartUI SFC compiler sample

This sample exercises the experimental single-file component syntax in the
KartUI compiler and VS Code grammar checks. `menu.kui` supplies the exported
component name `Menu`; its Go script uses a `setup` function, and the compiler
emits the component API from the file name and setup parameters.

From the repository root, print the generated Go component with:

```sh
go run ./samples/sfc-demo
```

The command compiles `menu.kui` together with the `main-menu.kui` layout and
writes the generated `Menu(args MenuProps)` function to stdout. To save it,
redirect the output, for example `go run ./samples/sfc-demo > /tmp/menu.go`.
Run `mise run test` to run the broader compiler and editor checks against the
sample sources.

The Go script is valid Go after adding a package declaration. This sample does
not yet connect VS Code completion requests to `gopls`.
