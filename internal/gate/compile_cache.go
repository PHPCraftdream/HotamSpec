package gate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// Compilation is shared only within one execution session. Its content/profile
// key cannot alias a concurrently compiling older generation of the same package.
type compileCacheKey struct {
	moduleRoot      string
	pkgPattern      string
	coverPkgPattern string
	inputHash       string
	profile         string
}

type compiledBinary struct {
	path          string
	compileFailed bool
	output        string
	err           error
}

type compileInFlightCall struct {
	done chan struct{}
	bin  *compiledBinary
}

var compileInvocations int64
var compileBinarySeq int64

func CompileInvocationCount() int64 { return atomic.LoadInt64(&compileInvocations) }

// ResetCompileInvocationCountForTest resets diagnostics, never an owner's state.
func ResetCompileInvocationCountForTest() {
	atomic.StoreInt64(&compileInvocations, 0)
}

func (s *ExecutionSession) syncCompileCacheToHash(moduleRoot, hash string) {
	cleaned := filepath.Clean(moduleRoot)
	s.compileModuleHashMu.Lock()
	defer s.compileModuleHashMu.Unlock()
	if previous, ok := s.compileModuleHash[cleaned]; ok && previous != hash {
		s.compileCache.Range(func(key, _ any) bool {
			if key.(compileCacheKey).moduleRoot == cleaned {
				s.compileCache.Delete(key)
			}
			return true
		})
	}
	s.compileModuleHash[cleaned] = hash
}

func (s *ExecutionSession) compileTestBinary(ctx context.Context, moduleRoot, pkgPattern, coverPkgPattern string) *compiledBinary {
	hash, err := hashPackageInputs(moduleRoot, packageDirFromPattern(moduleRoot, pkgPattern))
	if err != nil {
		return &compiledBinary{err: fmt.Errorf("hash compilation inputs: %w", err)}
	}
	s.syncCompileCacheToHash(moduleRoot, hash)
	key := compileCacheKey{filepath.Clean(moduleRoot), pkgPattern, coverPkgPattern, hash, executionProfile()}
	if cached, ok := s.compileCache.Load(key); ok {
		return cached.(*compiledBinary)
	}
	s.compileSingleflightMu.Lock()
	// Recheck under the flight lock: a preceding compiler may have published
	// between the optimistic lookup and registering this caller.
	if cached, ok := s.compileCache.Load(key); ok {
		s.compileSingleflightMu.Unlock()
		return cached.(*compiledBinary)
	}
	if existing, ok := s.compileInFlight[key]; ok {
		s.compileSingleflightMu.Unlock()
		select {
		case <-existing.done:
			return existing.bin
		case <-ctx.Done():
			return &compiledBinary{err: fmt.Errorf("go test -c for %s: %w (caller cancelled while waiting for an in-flight compile)", pkgPattern, ctx.Err())}
		}
	}
	call := &compileInFlightCall{done: make(chan struct{})}
	s.compileInFlight[key] = call
	s.compileSingleflightMu.Unlock()
	bin := s.doCompileTestBinary(moduleRoot, pkgPattern, coverPkgPattern)
	call.bin = bin
	if bin.err == nil {
		s.compileCache.Store(key, bin)
	}
	s.compileSingleflightMu.Lock()
	delete(s.compileInFlight, key)
	close(call.done)
	s.compileSingleflightMu.Unlock()
	return bin
}

const compileTimeout = 180 * time.Second

func (s *ExecutionSession) doCompileTestBinary(moduleRoot, pkgPattern, coverPkgPattern string) *compiledBinary {
	if err := s.ensureCompileTmpDir(); err != nil {
		return &compiledBinary{err: fmt.Errorf("could not create compile-cache tmp dir: %w", err)}
	}
	seq := atomic.AddInt64(&compileBinarySeq, 1)
	binaryPath := filepath.Join(s.compileTmpDir, compileBinaryName(moduleRoot, pkgPattern, coverPkgPattern, seq))
	ctx, cancel := context.WithTimeout(context.Background(), compileTimeout)
	defer cancel()
	args := []string{"test", "-c", "-vet=off", "-o", binaryPath, "-count=1"}
	if coverPkgPattern != "" {
		args = append(args, "-coverpkg="+coverPkgPattern)
	}
	args = append(args, pkgPattern)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = moduleRoot
	var buf limitedExecutionBuffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	atomic.AddInt64(&compileInvocations, 1)
	err := cmd.Run()
	output := boundOutput(buf.String())
	if buf.overflow {
		return &compiledBinary{output: output, err: fmt.Errorf("compile output exceeds %d bytes", maxExecutionOutput)}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return &compiledBinary{output: output, err: fmt.Errorf("go test -c timed out compiling %s in %s: %w", pkgPattern, moduleRoot, ctx.Err())}
	}
	if err != nil {
		var exitErr *exec.ExitError
		if !isExitError(err, &exitErr) {
			return &compiledBinary{output: output, err: fmt.Errorf("could not run go test -c: %w", err)}
		}
		return &compiledBinary{compileFailed: true, output: output}
	}
	if _, err := os.Stat(binaryPath); err != nil {
		return &compiledBinary{output: output, err: fmt.Errorf("go test -c reported success but no compiled binary at %s: %w", binaryPath, err)}
	}
	return &compiledBinary{path: binaryPath, output: output}
}

// Unique filenames retain binaries already held by running users after a source
// change. Only Close removes them, after all session users have stopped.
func compileBinaryName(moduleRoot, pkgPattern, coverPkgPattern string, seq int64) string {
	h := sha256.New()
	h.Write([]byte(moduleRoot + "\n"))
	h.Write([]byte(pkgPattern + "\n"))
	h.Write([]byte(coverPkgPattern + "\n"))
	return hex.EncodeToString(h.Sum(nil)) + "-" + fmt.Sprintf("%d", seq) + ".test"
}

func packageDirFromPattern(moduleRoot, pkgPattern string) string {
	rel := strings.TrimSuffix(strings.TrimPrefix(pkgPattern, "./"), "/")
	if rel == "" {
		return moduleRoot
	}
	return filepath.Join(moduleRoot, filepath.FromSlash(rel))
}
