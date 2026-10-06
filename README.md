# KartUI

**Typed, declarative game interfaces for Karty.**

KartUI combines markup, Go expressions, reusable components, and styles to
make game menus and HUDs easy to author.

A `main-menu.kui` file declares a component with typed inputs and Go callbacks:

```html
<script setup lang="go">
type MainMenuProps struct {
    Title string
    Play func()
}

func setup(props MainMenuProps) {}
</script>

<template>
    <panel modal="true">
        <label class="title">{ props.Title }</label>
        <button onClick={ props.Play }>Play</button>
    </panel>
</template>

<style>
.title
  font-size: theme.typography.title
</style>
```

The file name supplies the component name `MainMenu`; the Go setup parameters
define its inputs. `props.Title` supplies the label text, and `props.Play` runs
when the button is activated. Setup logic runs once per mounted instance;
the script and style blocks are optional.

- **Compiler** for components, layouts, themes, and styles.
- **Schema** for bounded UI definitions shared with the runtime.
- **Go adapters** for typed bindings and callbacks.
- **VS Code extension** for context-aware completion, hover help, local style
  navigation, highlighting and snippets.

This single-file component format is experimental. The compiler also supports
the legacy `kartui Name(...) { ... }` form. See [the SFC sample](samples/sfc-demo/README.md)
for a component with setup logic, a reusable layout, and styles.
KartUI's compiler and editor support `.kui`. The CLI also discovers, stages and
packages `.kui` components/layouts alongside legacy `.ui` sources; existing
projects can keep their current format.

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
