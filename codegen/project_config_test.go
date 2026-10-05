package codegen

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"math"
	"testing"
)

func TestProjectConfigFileTypedConstants(t *testing.T) {
	t.Parallel()

	configuration := ProjectConfiguration{ResolutionWidth: 1280, ResolutionHeight: 720,
		CameraFOVY: float32(math.Pi / 2), CameraNear: .05, CameraFar: 80, CameraOrthoHeight: 22}

	source, err := ProjectConfigFile(configuration)
	if err != nil {
		t.Fatal(err)
	}

	files := token.NewFileSet()

	parsed, err := parser.ParseFile(files, "project.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}

	checker := types.Config{}

	generated, err := checker.Check("example.com/game/.karty/config", files, []*ast.File{parsed}, nil)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		name  string
		kind  types.BasicKind
		value float64
	}{
		{"ResolutionWidth", types.Uint32, 1280}, {"ResolutionHeight", types.Uint32, 720},
		{"CameraFOVY", types.Float32, float64(configuration.CameraFOVY)},
		{"CameraNear", types.Float32, float64(configuration.CameraNear)},
		{"CameraFar", types.Float32, 80}, {"CameraOrthoHeight", types.Float32, 22},
	} {
		object, isConstant := generated.Scope().Lookup(test.name).(*types.Const)
		if !isConstant {
			t.Fatalf("generated %s must be a constant", test.name)
		}

		basic, ok := object.Type().(*types.Basic)
		if !ok || basic.Kind() != test.kind {
			t.Fatalf("generated %s must have the camera-compatible type", test.name)
		}

		value, _ := constant.Float64Val(constant.ToFloat(object.Val()))
		if value != test.value {
			t.Fatalf("%s=%v, expected %v", test.name, value, test.value)
		}
	}
}
