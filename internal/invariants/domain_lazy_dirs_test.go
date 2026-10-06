package invariants

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

func TestCheckDomainDirsLazyMaterialized(t *testing.T) {
	cases := []struct {
		name      string
		build     func(root string)
		wantCount int
	}{
		{name: "no dirs is correct", build: func(string) {}, wantCount: 0},
		{name: "agents with real sub-agent crystal", build: func(root string) {
			lazyDirWrite(t, filepath.Join(root, "agents", "scout", "CLAUDE.md"), "# scout\n")
		}, wantCount: 0},
		{name: "empty agents dir is an eager scaffold", build: func(root string) {
			lazyDirMkdir(t, filepath.Join(root, "agents"))
		}, wantCount: 1},
		{name: "agents dir with a stray file but no crystal", build: func(root string) {
			lazyDirWrite(t, filepath.Join(root, "agents", "notes.txt"), "x")
		}, wantCount: 1},
		{name: "tools with a real file", build: func(root string) {
			lazyDirWrite(t, filepath.Join(root, "tools", "lint.sh"), "#!/bin/sh\n")
		}, wantCount: 0},
		{name: "empty tools dir is an eager scaffold", build: func(root string) {
			lazyDirMkdir(t, filepath.Join(root, "tools"))
		}, wantCount: 1},
		{name: "tools with only dotfiles is an eager scaffold", build: func(root string) {
			lazyDirWrite(t, filepath.Join(root, "tools", ".keep"), "")
		}, wantCount: 1},
		{name: "both dirs wrong fires twice", build: func(root string) {
			lazyDirMkdir(t, filepath.Join(root, "agents"))
			lazyDirMkdir(t, filepath.Join(root, "tools"))
		}, wantCount: 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.build(root)
			got := checkDomainDirsLazyMaterialized(&ontology.Graph{DomainDir: root})
			if len(got) != tc.wantCount {
				t.Fatalf("got %d violations, want %d: %v", len(got), tc.wantCount, got)
			}
			for _, v := range got {
				if v.Check != "check_domain_dirs_lazy_materialized" {
					t.Errorf("violation check = %q, want check_domain_dirs_lazy_materialized", v.Check)
				}
			}
		})
	}
}

func TestCheckDomainDirsLazyMaterialized_EmptyDomainDirNoop(t *testing.T) {
	if got := checkDomainDirsLazyMaterialized(&ontology.Graph{}); len(got) != 0 {
		t.Fatalf("empty DomainDir must be a no-op (synthetic fixture graphs), got %v", got)
	}
}

func lazyDirMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func lazyDirWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
