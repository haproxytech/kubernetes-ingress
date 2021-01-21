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
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
)

const (
	LogTypeShort = log.LstdFlags
	LogType      = log.LstdFlags | log.Lshortfile
)

type LogLevel int8

const (
	Panic   LogLevel = 1
	Error   LogLevel = 2
	Warning LogLevel = 3
	Info    LogLevel = 4
	Debug   LogLevel = 5
	Trace   LogLevel = 6
)

// Logger provides functions to writing log messages
// level can be defined only as `trace`, `debug`, `info`, `warning`, `error`
// error and panic are always printed, panic also exits application.
//
// if nil is sent, it won't be printed. This is useful for printing errors only
// if they exist.
// ```
// if err != nil {
//   logger.Error(err)
// }
// ```
// can be shortened to
// ```
// logger.Error(err)
// ```
type Logger interface {
	Print(args ...interface{}) // always print regardless of Log level
	Trace(args ...interface{}) // used for heavy duty output everything, not recommended for production
	Debug(args ...interface{}) // used to have detailed output of application flow
	Info(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
	Err(args ...interface{}) []error
	Panic(args ...interface{})

	Printf(format string, args ...interface{})   // similar to fmt.SPrintf function
	Tracef(format string, args ...interface{})   // similar to fmt.SPrintf function
	Debugf(format string, args ...interface{})   // similar to fmt.SPrintf function
	Infof(format string, args ...interface{})    // similar to fmt.SPrintf function
	Warningf(format string, args ...interface{}) // similar to fmt.SPrintf function
	Errorf(format string, args ...interface{})   // similar to fmt.SPrintf function
	Panicf(format string, args ...interface{})   // similar to fmt.SPrintf function

	SetLevel(level LogLevel)
	ShowFilename(show bool)
}

type logger struct {
	Level    LogLevel
	FileName bool
}

var logSingelton *logger
var doOnce sync.Once

var k8sAPILogSingelton *logger
var dok8sAPIOnce sync.Once

func GetLogger() *logger { //nolint - 'exported func GetLogger returns unexported type , which can be annoying to use' - this is deliberate here
	doOnce.Do(func() {
		logSingelton = &logger{
			Level:    Warning,
			FileName: true,
		}
		log.SetFlags(LogTypeShort)
	})
	return logSingelton
}

func GetK8sAPILogger() *logger { //nolint - 'exported func GetLogger returns unexported type , which can be annoying to use' - this is deliberate here
	dok8sAPIOnce.Do(func() {
		k8sAPILogSingelton = &logger{
			Level:    Trace,
			FileName: true,
		}
	})
	return k8sAPILogSingelton
}

func (l *logger) SetLevel(level LogLevel) {
	l.Level = level
}

func (l *logger) ShowFilename(show bool) {
	l.FileName = show
}

func (l *logger) log(logType string, data ...interface{}) {
	if !l.FileName {
		for _, d := range data {
			if d == nil {
				continue
			}
			log.Printf("%s%s\n", logType, d)
		}
		return
	}
	_, file, no, ok := runtime.Caller(2)
	if ok {
		f := strings.Split(file, "/")
		var file1 string
		if f[len(f)-2] == "controller" || f[len(f)-2] == "kubernetes-ingress" {
			file1 = f[len(f)-1]
		} else {
			file1 = fmt.Sprintf("%s/%s", f[len(f)-2], f[len(f)-1])
		}
		// file1 := strings.Replace(file, "/src/", "", 1)
		for _, d := range data {
			if d == nil {
				continue
			}

			if logType == "" {
				log.Printf("%s:%d %s\n", file1, no, d)
			} else {
				log.Printf("%s%s:%d %s\n", logType, file1, no, d)
			}
		}
	}

}

func (l *logger) logf(logType string, format string, data ...interface{}) {
	line := fmt.Sprintf(format, data...)
	if !l.FileName {
		log.Printf("%s%s\n", logType, line)
		return
	}
	_, file, no, ok := runtime.Caller(2)
	if ok {
		f := strings.Split(file, "/")
		var file1 string
		if f[len(f)-2] == "controller" || f[len(f)-2] == "kubernetes-ingress" {
			file1 = f[len(f)-1]
		} else {
			file1 = fmt.Sprintf("%s/%s", f[len(f)-2], f[len(f)-1])
		}
		// file1 := strings.Replace(file, "/src/", "", 1)
		if logType == "" {
			log.Printf("%s:%d %s\n", file1, no, line)
		} else {
			log.Printf("%s%s:%d %s\n", logType, file1, no, line)
		}
	}

}

func (l *logger) Print(args ...interface{}) {
	l.log("", args...)
}

func (l *logger) Printf(format string, args ...interface{}) {
	l.logf("", format, args...)
}

func (l *logger) Trace(args ...interface{}) {
	if l.Level >= Trace {
		l.log("TRACE   ", args...)
	}
}

func (l *logger) Tracef(format string, args ...interface{}) {
	if l.Level >= Trace {
		l.logf("TRACE   ", format, args...)
	}
}

func (l *logger) Debug(args ...interface{}) {
	if l.Level >= Debug {
		l.log("DEBUG   ", args...)
	}
}

func (l *logger) Debugf(format string, args ...interface{}) {
	if l.Level >= Debug {
		l.logf("DEBUG   ", format, args...)
	}
}

func (l *logger) Info(args ...interface{}) {
	if l.Level >= Info {
		l.log("INFO    ", args...)
	}
}

func (l *logger) Infof(format string, args ...interface{}) {
	if l.Level >= Info {
		l.logf("INFO    ", format, args...)
	}
}

func (l *logger) Warning(args ...interface{}) {
	if l.Level >= Warning {
		l.log("WARNING ", args...)
	}
}

func (l *logger) Warningf(format string, args ...interface{}) {
	if l.Level >= Warning {
		l.logf("WARNING ", format, args...)
	}
}

func (l *logger) Err(args ...interface{}) []error {
	// showing errors can't be disabled
	l.log("ERROR   ", args...)
	result := []error{}
	for _, d := range args {
		if d == nil {
			continue
		}
		err, ok := d.(error)
		if ok {
			result = append(result, err)
		}
	}
	if len(result) > 0 {
		return result
	}
	return nil
}

func (l *logger) Error(args ...interface{}) {
	// showing errors can't be disabled
	l.log("ERROR   ", args...)
}

func (l *logger) Errorf(format string, args ...interface{}) {
	// showing errors can't be disabled
	l.logf("ERROR   ", format, args...)
}

func (l *logger) Panic(args ...interface{}) {
	l.log("PANIC   ", args...)
	for _, val := range args {
		if val != nil {
			panic(val)
		}
	}
}

func (l *logger) Panicf(format string, args ...interface{}) {
	l.logf("PANIC   ", format, args...)
	for _, val := range args {
		if val != nil {
			panic(val)
		}
	}
}
