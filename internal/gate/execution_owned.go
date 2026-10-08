package gate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
)

// One-shot entry points own and close a complete session before returning.
// Callers collecting several projections use an explicit ExecutionSession.
func RunVerifiedByTest(specRoot, file, testName string) (result TestRunResult) {
	s := NewExecutionSession()
	defer func() { result.Err = errors.Join(result.Err, s.Close()) }()
	return s.RunVerifiedByTest(specRoot, file, testName)
}

func RunVerifiedByTestRecording(specRoot, file, testName, coverPkgFile string) (result RecordingResult) {
	s := NewExecutionSession()
	defer func() { result.Err = errors.Join(result.Err, s.Close()) }()
	return s.RunVerifiedByTestRecording(specRoot, file, testName, coverPkgFile)
}

func RunAtomPackageRecording(specRoot, file string) (result RecordingResult) {
	s := NewExecutionSession()
	defer func() { result.Err = errors.Join(result.Err, s.Close()) }()
	return s.RunAtomPackageRecording(specRoot, file)
}

func RunAtomTestRecording(specRoot, file, testName, coverPkgFile string) (result RecordingResult) {
	s := NewExecutionSession()
	defer func() { result.Err = errors.Join(result.Err, s.Close()) }()
	return s.RunAtomTestRecording(specRoot, file, testName, coverPkgFile)
}

const maxExecutionOutput = 64 << 20

type limitedExecutionBuffer struct {
	buffer   bytes.Buffer
	overflow bool
}

func (b *limitedExecutionBuffer) Len() int       { return b.buffer.Len() }
func (b *limitedExecutionBuffer) Bytes() []byte  { return b.buffer.Bytes() }
func (b *limitedExecutionBuffer) String() string { return b.buffer.String() }

func (b *limitedExecutionBuffer) Write(data []byte) (int, error) {
	count := len(data)
	remaining := maxExecutionOutput - b.Len()
	if len(data) > remaining {
		data = data[:remaining]
		b.overflow = true
	}
	_, _ = b.buffer.Write(data)
	return count, nil
}

func readExecutionFile(path string, maximum int) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, int64(maximum)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maximum {
		return nil, fmt.Errorf("execution artifact %q exceeds remaining %d-byte capture limit", path, maximum)
	}
	return data, nil
}
