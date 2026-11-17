package pico

import (
	"strings"
	"testing"
)

func TestResolveComponentPathFindsImport(t *testing.T) {
	components := []Component{{Name: "Icon", Path: "layouts/components/icons.pico"}}
	path, err := resolveComponentPath(components, "Icon", "layouts/global/footer.pico")
	if err != nil {
		t.Fatalf("resolve imported component: %v", err)
	}
	if path != "layouts/components/icons.pico" {
		t.Fatalf("unexpected component path: %q", path)
	}
}

func TestResolveComponentPathReportsMissingImport(t *testing.T) {
	_, err := resolveComponentPath(nil, "Icon", "layouts/global/footer.pico")
	if err == nil {
		t.Fatal("expected an error for an unimported component")
	}
	for _, want := range []string{
		"Component <Icon>",
		"layouts/global/footer.pico",
		`import Icon from "...";`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}
