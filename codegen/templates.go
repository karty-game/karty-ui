package codegen

import _ "embed"

//go:embed templates/ui-views.go.tmpl
var uiViewsSource string

//go:embed templates/ui-client.go.tmpl
var uiClientSource string

//go:embed templates/ui-composed-client.go.tmpl
var uiComposedClientSource string

//go:embed templates/assets.go.tmpl
var assetsSourceTemplate string

//go:embed templates/project-config.go.tmpl
var projectConfigSource string
