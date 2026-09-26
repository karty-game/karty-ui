package uicompiler

import (
	"os"
	"strings"
	"testing"
)

func TestSDK011StyleReferenceCoversEveryAcceptedProperty(t *testing.T) {
	t.Parallel()

	contents, err := os.ReadFile("../docs/0.0.1/styles.md")
	if err != nil {
		t.Fatal(err)
	}

	for _, property := range supportedStylePropertyNames() {
		if !strings.Contains(string(contents), "`"+property+"`") {
			t.Fatalf("SDK-local style reference omits accepted property %q", property)
		}
	}
}
