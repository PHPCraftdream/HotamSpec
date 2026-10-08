package gate

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if err := CloseExecutionSessions(); err != nil {
		fmt.Fprintf(os.Stderr, "compile-cache cleanup: %v\n", err)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}
