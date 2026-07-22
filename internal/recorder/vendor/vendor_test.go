package vendor

import (
	"strings"
	"testing"
)

func TestSource_CarriesBannerThenCanonBody(t *testing.T) {
	full := Source()
	if !strings.HasPrefix(full, Banner) {
		t.Fatalf("Source() does not start with Banner")
	}
	body := strings.TrimPrefix(full, Banner)
	if body != BodyForHash() {
		t.Fatalf("Source() body after banner does not equal BodyForHash()")
	}
	if !strings.Contains(full, "package hotamspec") {
		t.Fatalf("vendored source does not declare package hotamspec")
	}
	if !strings.Contains(full, "DO NOT EDIT") {
		t.Fatalf("vendored source banner missing DO NOT EDIT marker")
	}
}

func TestStripBanner_RoundTrips(t *testing.T) {
	full := Source()
	body, ok := StripBanner(full)
	if !ok {
		t.Fatalf("StripBanner(vendored source) ok = false, want true")
	}
	if body != BodyForHash() {
		t.Fatalf("StripBanner body mismatch")
	}

	tampered := "package hotamspec\n\n// no banner here\n"
	if _, ok := StripBanner(tampered); ok {
		t.Fatalf("StripBanner(no-banner content) ok = true, want false")
	}
}

func TestSource_Deterministic(t *testing.T) {
	if Source() != Source() {
		t.Fatalf("Source() is not deterministic across calls")
	}
}
