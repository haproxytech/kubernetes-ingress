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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWrite(t *testing.T) {
	var counter atomic.Int32
	w := New()
	w.Write(func() {
		counter.Add(1)
	})
	w.Write(func() {
		time.Sleep(time.Second)
		counter.Add(1)
	})
	w.WaitUntilWritesDone()

	assert.Equal(t, int32(2), counter.Load())
}

func TestWriteSecondTime(t *testing.T) {
	var counter atomic.Int32
	w := New()
	w.Write(func() {
		counter.Add(1)
	})
	w.Write(func() {
		counter.Add(1)
	})
	w.WaitUntilWritesDone()

	assert.Equal(t, int32(2), counter.Load())

	counter.Store(0)
	w.Write(func() {
		time.Sleep(time.Second)
		counter.Add(1)
	})
	w.Write(func() {
		counter.Add(1)
	})
	w.WaitUntilWritesDone()

	assert.Equal(t, int32(2), counter.Load())
}

func TestWriteSecondQueue(t *testing.T) {
	var counter atomic.Int32
	w := New()
	numWrites := int32(FS_WRITE_LIMIT*2 + 2)

	for range numWrites {
		w.Write(func() {
			counter.Add(1)
		})
	}
	w.WaitUntilWritesDone()

	assert.Equal(t, numWrites, counter.Load())

	counter.Store(0)
	for range numWrites {
		w.Write(func() {
			counter.Add(1)
		})
	}
	w.WaitUntilWritesDone()

	assert.Equal(t, numWrites, counter.Load())
}

func TestWriteSecondQueueTime(t *testing.T) {
	var counter atomic.Int32
	w := New()
	numWrites := FS_WRITE_LIMIT*2 + 2

	start := time.Now()
	for range numWrites {
		w.Write(func() {
			time.Sleep(time.Second)
			counter.Add(1)
		})
	}
	w.WaitUntilWritesDone()
	diffTime := time.Since(start)

	if counter.Load() != int32(numWrites) {
		t.Errorf("expected %d writes, got %d", numWrites, counter.Load())
	}

	numSeconds := numWrites/FS_WRITE_LIMIT + 1

	assert.Less(t, time.Second*time.Duration(numSeconds), diffTime)
	assert.Less(t, diffTime, time.Second*time.Duration(numSeconds+1))
}
