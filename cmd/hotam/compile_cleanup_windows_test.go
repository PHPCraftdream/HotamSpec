package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
)

func TestCLICleanupFailureChangesSuccessButPreservesUsageError(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a real fixture binary and holds a Windows deletion lock")
	}
	directories := populateCLICompileCache(t)
	files, err := filepath.Glob(filepath.Join(directories[0], "*.test"))
	if err != nil || len(files) == 0 {
		t.Fatalf("compiled fixture binary is absent: %v %v", files, err)
	}
	name, err := syscall.UTF16PtrFromString(files[0])
	if err != nil {
		t.Fatal(err)
	}
	handle, err := syscall.CreateFile(name, syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.CloseHandle(handle); err != nil {
			t.Error(err)
		}
		if err := gate.CloseExecutionSessions(); err != nil {
			t.Error(err)
		}
	})
	if code := runCLI([]string{"version"}); code != 1 {
		t.Fatalf("cleanup failure was reported as success: %d", code)
	}
	if _, err := os.Stat(files[0]); err != nil {
		t.Fatalf("locked artifact was lost: %v", err)
	}
	if code := runCLI(nil); code != 2 {
		t.Fatalf("cleanup failure replaced the original usage exit code: %d", code)
	}
}
