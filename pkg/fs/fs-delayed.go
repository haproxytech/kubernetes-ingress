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

var (
	delayedFunc map[string]func()
	muDelayed   sync.Mutex
)
var delayedWriter = New()

// AddDelayedFunc adds a function to be called prior to restarting of HAProxy
func AddDelayedFunc(name string, f func()) {
	muDelayed.Lock()
	defer muDelayed.Unlock()
	if delayedFunc == nil {
		delayedFunc = make(map[string]func())
	}
	delayedFunc[name] = f
}

func RunDelayedFuncs() {
	muDelayed.Lock()
	defer muDelayed.Unlock()
	if delayedFunc == nil {
		return
	}
	for _, f := range delayedFunc {
		delayedWriter.Write(f)
	}
	clear(delayedFunc)
	delayedWriter.WaitUntilWritesDone()
}
