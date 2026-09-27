package codegen

import (
	"bytes"
	"testing"
)

func TestTextureAssetPackageFileQualifiesEngineType(t *testing.T) {
	t.Parallel()

	generated, err := TextureAssetPackageFile([]string{"sprites.player"}, "example.com/game/.karty/engine")
	if err != nil {
		t.Fatal(err)
	}

	for _, expected := range [][]byte{
		[]byte("package assets"),
		[]byte(`engine "example.com/game/.karty/engine"`),
		[]byte(`TextureSpritesPlayer engine.TextureID = "sprites.player"`),
	} {
		if !bytes.Contains(generated, expected) {
			t.Fatalf("generated package missing %q:\n%s", expected, generated)
		}
	}
}

func TestTextureAssetPackageFileEmptyHasNoUnusedImport(t *testing.T) {
	t.Parallel()

	generated, err := TextureAssetPackageFile(nil, "example.com/game/.karty/engine")
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Contains(generated, []byte("import engine")) {
		t.Fatalf("empty generated package has unused import:\n%s", generated)
	}
}
