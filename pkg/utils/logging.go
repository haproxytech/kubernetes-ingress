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

//nolint:stylecheck
const (
	LogTypeShort       = log.LstdFlags
	LogType            = log.LstdFlags | log.Lshortfile
	PKG_SUBPATH        = "/pkg/"
	CONTROLLER_SUBPATH = "/controller"
	IC_REPO            = "kubernetes-ingress/"
	IC_EE_REPO         = "kubernetes-ingress-ee/"
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

const (
	LogFieldTransactionID = "transactionID"
)

// Logger provides functions to writing log messages
// level can be defined only as `trace`, `debug`, `info`, `warning`, `error`
// error and panic are always printed, panic also exits application.
//
// if nil is sent, it won't be printed. This is useful for printing errors only
// if they exist.
//
// ```
//
//	if err != nil {
//	  logger.Error(err)
//	}
//
// ```
// can be shortened to
// ```
// logger.Error(err)
// ```
type Logger interface { //nolint:interfacebloat
	Print(args ...interface{}) // always print regardless of Log level
	Trace(args ...interface{}) // used for heavy duty output everything, not recommended for production
	Debug(args ...interface{}) // used to have detailed output of application flow
	Info(args ...interface{})
	InfoSkipCaller(args ...interface{})
	Warning(args ...interface{})
	Error(args ...interface{})
	Err(args ...interface{}) []error
	Panic(args ...interface{})

	Printf(format string, args ...interface{})          // similar to fmt.SPrintf function
	Tracef(format string, args ...interface{})          // similar to fmt.SPrintf function
	Debugf(format string, args ...interface{})          // similar to fmt.SPrintf function
	Infof(format string, args ...interface{})           // similar to fmt.SPrintf function
	InfoSkipCallerf(format string, args ...interface{}) // similar to fmt.SPrintf function
	Warningf(format string, args ...interface{})        // similar to fmt.SPrintf function
	Errorf(format string, args ...interface{})          // similar to fmt.SPrintf function
	Panicf(format string, args ...interface{})          // similar to fmt.SPrintf function

	SetLevel(level LogLevel)
	ShowFilename(show bool)
	WithField(key string, value interface{})
	ResetFields()
}

type logger struct {
	fields   map[string]interface{}
	Level    LogLevel
	FileName bool
}

var (
	logSingelton *logger
	doOnce       sync.Once
)

var (
	k8slogSingelton *logger
	k8sdoOnce       sync.Once
)

//nolint:golint // 'exported func GetLogger returns unexported type , which can be annoying to use' - this is deliberate here
func GetLogger() *logger {
	doOnce.Do(func() {
		logSingelton = &logger{
			Level:    Warning,
			FileName: true,
			fields:   make(map[string]interface{}),
		}
		log.SetFlags(LogTypeShort)
	})
	return logSingelton
}

//nolint:golint // 'exported func GetK8sLogger returns unexported type , which can be annoying to use' - this is deliberate here
func GetK8sLogger() *logger {
	k8sdoOnce.Do(func() {
		k8slogSingelton = &logger{
			Level:    Warning,
			FileName: true,
			fields:   make(map[string]interface{}),
		}
		log.SetFlags(LogTypeShort)
	})
	return k8slogSingelton
}

func (l *logger) SetLevel(level LogLevel) {
	l.Level = level
}

func (l *logger) ShowFilename(show bool) {
	l.FileName = show
}

func (l *logger) WithField(key string, value interface{}) {
	l.fields[key] = value
}

func (l *logger) ResetFields() {
	clear(l.fields)
}

func (l *logger) fieldsAsString() string {
	var fields strings.Builder
	if len(l.fields) > 0 {
		fields.WriteString("[")
	}
	for k, v := range l.fields {
		fields.WriteString(fmt.Sprintf("%s=%v", k, v))
	}
	if len(l.fields) > 0 {
		fields.WriteString("]")
	}

	return fields.String()
}

func (l *logger) logSkipCaller(logType string, data ...interface{}) {
	l.logImp(logType, 2, data...)
}

func (l *logger) log(logType string, data ...interface{}) {
	l.logImp(logType, 0, data...)
}

func (l *logger) logImp(logType string, skipMore int, data ...interface{}) {
	if !l.FileName {
		for _, d := range data {
			if d == nil {
				continue
			}
			log.Printf("%s%s\n", logType, d)
		}
		return
	}
	_, file, no, ok := runtime.Caller(3 + skipMore)
	if ok {
		origin := getFileCaller(file)

		for _, d := range data { //nolint:varnamelen
			if d == nil {
				continue
			}

			if logType == "" {
				log.Printf("%s:%d %s %s\n", origin, no, l.fieldsAsString(), d)
			} else {
				log.Printf("%s%s:%d %s %s\n", logType, origin, no, l.fieldsAsString(), d)
			}
		}
	}
}

func (l *logger) logfSkipCallerf(logType string, format string, data ...interface{}) {
	l.logfImpl(logType, format, 2, data...)
}

func (l *logger) logf(logType string, format string, data ...interface{}) {
	l.logfImpl(logType, format, 0, data...)
}

func (l *logger) logfImpl(logType string, format string, skipMore int, data ...interface{}) {
	line := fmt.Sprintf(format, data...)
	if !l.FileName {
		log.Printf("%s%s\n", logType, line)
		return
	}
	_, file, no, ok := runtime.Caller(3 + skipMore)

	if ok {
		origin := getFileCaller(file)
		if logType == "" {
			log.Printf("%s:%d %s %s\n", origin, no, l.fieldsAsString(), line)
		} else {
			log.Printf("%s%s:%d %s %s\n", logType, origin, no, l.fieldsAsString(), line)
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

func (l *logger) InfoSkipCaller(args ...interface{}) {
	if l.Level >= Info {
		l.logSkipCaller("INFO    ", args...)
	}
}

func (l *logger) Infof(format string, args ...interface{}) {
	if l.Level >= Info {
		l.logf("INFO    ", format, args...)
	}
}

func (l *logger) InfoSkipCallerf(format string, args ...interface{}) {
	if l.Level >= Info {
		l.logfSkipCallerf("INFO    ", format, args...)
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

func (l *logger) HandleWarningHeader(code int, agent string, text string) {
	if code == 299 {
		switch {
		case strings.Contains(text, "use core.haproxy.org/v1alpha2 Defaults"),
			strings.Contains(text, "use core.haproxy.org/v1alpha2 Global"),
			strings.Contains(text, "use core.haproxy.org/v1alpha2 Backend"):
			return
		}
	}
	l.logf("K8s API ", " %d %s %s", code, agent, text)
}

func getFileCaller(path string) string {
	pkgIndex := strings.Index(path, PKG_SUBPATH)
	icIndex := strings.Index(path, CONTROLLER_SUBPATH)

	var origin string
	switch {
	case pkgIndex != -1:
		origin = path[pkgIndex+len(PKG_SUBPATH):]
	case icIndex != -1:
		origin = path[icIndex+len(CONTROLLER_SUBPATH):]
	default:
		offset := 0
		// do we have IC repo in path ?
		repoIndex := strings.Index(path, IC_REPO)
		if repoIndex == -1 {
			// if not do we have IC EE repo in path ?
			repoIndex = strings.Index(path, IC_EE_REPO)
			if repoIndex == -1 {
				// if not start reset the repo index to 0
				repoIndex = 0
			} else {
				// we have IC EE repo in path
				offset = len(IC_EE_REPO)
			}
		} else {
			// we have IC repo in path.
			offset = len(IC_REPO)
		}
		idx := repoIndex + offset
		origin = path[idx:]
	}
	return origin
}
