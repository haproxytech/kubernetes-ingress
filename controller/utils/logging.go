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

package utils

import (
	"log"
	"runtime"
	"strings"
)

const (
	LogTypeShort = log.LstdFlags
	LogType      = log.LstdFlags | log.Lshortfile
)

func LogErr(err error) {
	if err == nil {
		return
	}
	_, file, no, ok := runtime.Caller(1)
	if ok {
		file1 := strings.Replace(file, "/src/", "", 1)
		log.SetFlags(LogTypeShort)
		log.Printf("%s:%d %s\n", file1, no, err.Error())
		log.SetFlags(LogType)
	}
}

func PanicErr(err error) {
	if err == nil {
		return
	}
	_, file, no, ok := runtime.Caller(1)
	if ok {
		file1 := strings.Replace(file, "/src/", "", 1)
		log.SetFlags(LogTypeShort)
		log.Panicf("%s:%d %s\n", file1, no, err.Error())
	}
}
