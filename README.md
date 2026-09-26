# KartUI

**Typed, declarative game interfaces for Karty.**

KartUI combines markup, Go expressions, reusable components, and styles to
make game menus and HUDs easy to author.

A `.ui` file declares a component with typed inputs and Go callbacks:

```go
kartui MainMenu(Title string, Play func()) {
    <panel modal="true">
        <label>{ Title }</label>
        <button onClick={ Play }>Play</button>
    </panel>
}
```

`Title` supplies the label text; `Play` runs when the button is activated.
Components can also include setup logic, reusable layouts, and styles.

- **Compiler** for components, layouts, themes, and styles.
- **Schema** for bounded UI definitions shared with the runtime.
- **Go adapters** for typed bindings and callbacks.
- **VS Code extension** for highlighting and snippets.

This public repository builds independently of the private Karty Engine.
Rendering and platform input belong to the engine.

```sh
mise run test
mise run install-editor
mise run package-editor
```

[Language reference](docs/0.0.1/kartui.md) · [Styles](docs/0.0.1/styles.md) · [Development](docs/development.md) · [Releases](docs/releases.md)

Use KartUI in games with the [Karty CLI](https://github.com/karty-game/karty).

[Contributing](CONTRIBUTING.md) · [MIT license](LICENSE.md)
