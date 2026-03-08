package theme

import "testing"

func TestResolveDefaultsToDark(t *testing.T) {
	p, err := Resolve("")
	if err != nil {
		t.Fatalf("resolve default theme: %v", err)
	}
	if p.Name != "dark" {
		t.Fatalf("expected dark default, got %q", p.Name)
	}
}

func TestResolveRejectsUnknownTheme(t *testing.T) {
	if _, err := Resolve("nope"); err == nil {
		t.Fatal("expected unknown theme error")
	}
}
