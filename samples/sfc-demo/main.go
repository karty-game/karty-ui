// Command sfc-demo compiles the sample component and prints its generated Go.
package main

import (
	"fmt"
	"os"

	"github.com/karty-game/karty-ui/codegen"
	uicompiler "github.com/karty-game/karty-ui/compiler"
)

func main() {
	if err := generate(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate() error {
	root := "samples/sfc-demo"

	components, err := uicompiler.LoadProjectWithTheme(root,
		[]uicompiler.Source{{Name: "ui.menu", Source: "ui/views/menu.kui"}},
		[]uicompiler.LayoutSource{{Source: "ui/layouts/main-menu.kui"}}, "")
	if err != nil {
		return err
	}

	files, err := codegen.UIClientFiles(components, "example.com/sfc-demo")
	if err != nil {
		return err
	}

	if _, err := os.Stdout.Write(files[components[0].Source]); err != nil {
		return err
	}

	return nil
}
