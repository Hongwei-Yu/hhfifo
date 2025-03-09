package hhfifo

import (
	"runtime"
	"sync/atomic"
)

type highPerfSem struct {
	counter int32
}

func newHighPerfSem(permits int32) *highPerfSem {
	return &highPerfSem{counter: permits}
}

func (s *highPerfSem) Acquire() {
	for {
		if atomic.AddInt32(&s.counter, -1) >= 0 {
			return
		}
		atomic.AddInt32(&s.counter, 1)
		runtime.Gosched()
	}
}

func (s *highPerfSem) Release() {
	atomic.AddInt32(&s.counter, 1)
}
