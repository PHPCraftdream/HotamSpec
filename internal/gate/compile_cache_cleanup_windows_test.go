package gate

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestCompileCacheCleanupRetainsLockedDirectoryAndRetries(t *testing.T) {
	session := NewExecutionSession()
	root := t.TempDir()
	t.Setenv("TMP", root)
	t.Setenv("TEMP", root)
	if err := session.ensureCompileTmpDir(); err != nil {
		t.Fatal(err)
	}
	dir := session.compileTmpDir
	file := filepath.Join(dir, "held.test")
	if err := os.WriteFile(file, []byte("owned artifact"), 0600); err != nil {
		t.Fatal(err)
	}
	name, err := syscall.UTF16PtrFromString(file)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		if !closed {
			syscall.CloseHandle(handle)
		}
		if err := session.Close(); err != nil {
			t.Error(err)
		}
	})
	err = session.Close()
	if err == nil || !strings.Contains(err.Error(), dir) {
		t.Fatalf("locked artifact deletion was not reported with its directory: %v", err)
	}
	if session.compileTmpDir != dir {
		t.Fatalf("failed cleanup lost ownership: got %q want %q", session.compileTmpDir, dir)
	}
	if result := session.RunVerifiedByTest(root, "absent_test.go", "TestAbsent"); result.Err == nil {
		t.Fatal("failed Close accepted new execution")
	}
	if err := syscall.CloseHandle(handle); err != nil {
		t.Fatal(err)
	}
	closed = true
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("retry left the previously locked directory: %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("cleanup is not idempotent: %v", err)
	}
}
