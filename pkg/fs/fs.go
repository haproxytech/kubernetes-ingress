// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fs

import (
	"sync"
)

const FS_WRITE_LIMIT = 20 //nolint:stylecheck

var Writer = New()

type writer struct {
	writeLimiter chan struct{}
	wg           *sync.WaitGroup
	mu           *sync.Mutex
}

// New creates new writer that will parallelize fs writes
func New() writer {
	w := writer{
		writeLimiter: make(chan struct{}, FS_WRITE_LIMIT),
		wg:           &sync.WaitGroup{},
		mu:           &sync.Mutex{},
	}
	return w
}

// Write ensures function to be executed in a separate goroutine
// this also ensures that we do not put to much pressure on the FS
// while still allowing some parallelization
//
// NOTE: this will block calling of WaitUntilWritesDone
func (w *writer) Write(writeFunc func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wg.Add(1)
	go func() {
		defer func() {
			<-w.writeLimiter
			w.wg.Done()
		}()
		w.writeLimiter <- struct{}{}
		writeFunc()
	}()
}

// WaitUntilWritesDone waits for all fs writes to complete.
//
// NOTE: this will block calling of Write
func (w *writer) WaitUntilWritesDone() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.wg.Wait()
}
