package selfcheck_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/PHPCraftdream/HotamSpec/internal/gate"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if err := gate.CloseExecutionSessions(); err != nil {
		fmt.Fprintf(os.Stderr, "compile-cache cleanup: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
