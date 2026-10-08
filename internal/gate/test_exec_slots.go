package gate

// The process-wide semaphore bounds real execution across independent sessions.
// It carries no source, artifact or verdict state.
var globalExecSlots = make(chan struct{}, 2)

type cacheKey struct {
	pkgDir   string
	testName string
}

type cacheEntry struct {
	hash   string
	result TestRunResult
}

type inFlightCall struct {
	hash   string
	done   chan struct{}
	result TestRunResult
}

// A changed hash never shares an older run's verdict, even if it arrived while
// that run was in flight. There is no disk PASS cache or cross-session reuse.
func (s *ExecutionSession) singleflightRun(key cacheKey, hash string, run func() TestRunResult) TestRunResult {
	for {
		s.inFlightMu.Lock()
		if cached, ok := s.runCache.Load(key); ok && cached.(cacheEntry).hash == hash {
			s.inFlightMu.Unlock()
			return cached.(cacheEntry).result
		}
		existing, inFlight := s.inFlightCalls[key]
		if inFlight {
			s.inFlightMu.Unlock()
			<-existing.done
			if existing.hash == hash {
				return existing.result
			}
			continue
		}
		call := &inFlightCall{hash: hash, done: make(chan struct{})}
		s.inFlightCalls[key] = call
		s.inFlightMu.Unlock()
		call.result = run()
		s.inFlightMu.Lock()
		if call.result.Err == nil {
			s.runCache.Store(key, cacheEntry{hash: hash, result: call.result})
		}
		delete(s.inFlightCalls, key)
		close(call.done)
		s.inFlightMu.Unlock()
		return call.result
	}
}
