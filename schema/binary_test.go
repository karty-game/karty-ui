package ui

import (
	"encoding/binary"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBinaryTemplateRejectsJSONAndMalformedRecords(t *testing.T) {
	t.Parallel()

	value := Template{Version: SchemaSizing, Modal: true, Elements: []Element{{ID: 1, Name: "title", Kind: "label", Text: "é 🚀"}}}

	data, err := EncodeComposition(value)
	if err != nil {
		t.Fatal(err)
	}

	if string(data[:4]) != "KUI1" || binary.LittleEndian.Uint16(data[4:6]) != BinaryVersion {
		t.Fatal("missing binary version")
	}

	got, err := DecodeComposition(data)
	if err != nil || !reflect.DeepEqual(value, got) {
		t.Fatal(got, err)
	}

	for _, input := range [][]byte{[]byte(`{"version":11,"elements":[]}`), append(append([]byte(nil), data...), 0)} {
		if _, err := DecodeComposition(input); err == nil {
			t.Fatal("accepted JSON or trailing bytes")
		}
	}

	for _, offset := range []int{4, 6, 8, 12, 20} { // Version, flags, size, reserved, boolean.
		changed := append([]byte(nil), data...)
		changed[offset] = 255

		if _, err := DecodeComposition(changed); err == nil {
			t.Fatalf("accepted corrupt field at %d", offset)
		}
	}

	for size := range data {
		if _, err := DecodeComposition(data[:size]); err == nil {
			t.Fatalf("accepted truncation at %d", size)
		}
	}

	if _, err := decodeBinary(append([]byte("KUI1"), make([]byte, MaxAssetBytes)...)); err == nil {
		t.Fatal("accepted oversized record")
	}
}

func TestRuntimeSchemaHasNoJSONOrReflection(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}

		file, err := parser.ParseFile(token.NewFileSet(), name, source, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}

		for _, imported := range file.Imports {
			path := strings.Trim(imported.Path.Value, "\"")
			if path == "reflect" || strings.HasPrefix(path, "encoding/json") {
				t.Fatalf("runtime schema %s imports %s", name, path)
			}
		}
	}
}
