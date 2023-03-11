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
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
)

func HomeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}
	return os.Getenv("USERPROFILE") // windows
}

func Hash(input []byte) string {
	h := fnv.New128a()
	_, _ = h.Write(input)
	return hex.EncodeToString(h.Sum([]byte{}))
}

func PtrInt64(value int64) *int64 {
	return &value
}

func PtrString(value string) *string {
	return &value
}

func ParseInt(data string) (v int64, err error) {
	i, err := strconv.Atoi(data)
	if err == nil {
		v = int64(i)
	}
	return
}

func ParseTime(data string) (*int64, error) {
	var v int64
	var err error
	switch {
	case strings.HasSuffix(data, "ms"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "ms"), 10, 64)
	case strings.HasSuffix(data, "s"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "s"), 10, 64)
		v *= 1000
	case strings.HasSuffix(data, "m"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "m"), 10, 64)
		v = v * 1000 * 60
	case strings.HasSuffix(data, "h"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "h"), 10, 64)
		v = v * 1000 * 60 * 60
	case strings.HasSuffix(data, "d"):
		v, err = strconv.ParseInt(strings.TrimSuffix(data, "d"), 10, 64)
		v = v * 1000 * 60 * 60 * 24
	default:
		v, err = strconv.ParseInt(data, 10, 64)
	}
	return &v, err
}

func ParseSize(size string) (*int64, error) {
	var v int64
	var err error
	switch {
	case strings.HasSuffix(size, "k"):
		v, err = strconv.ParseInt(strings.TrimSuffix(size, "k"), 10, 64)
		v *= 1024
	case strings.HasSuffix(size, "m"):
		v, err = strconv.ParseInt(strings.TrimSuffix(size, "m"), 10, 64)
		v = v * 1024 * 1024
	case strings.HasSuffix(size, "g"):
		v, err = strconv.ParseInt(strings.TrimSuffix(size, "g"), 10, 64)
		v = v * 1024 * 1024 * 1024
	default:
		v, err = strconv.ParseInt(size, 10, 64)
	}
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func GetBoolValue(dataValue, dataName string) (result bool, err error) {
	result, err = strconv.ParseBool(dataValue)
	if err != nil {
		switch strings.ToLower(dataValue) {
		case "enabled", "on":
			logger := GetLogger()
			logger.Warningf(`%s - [%s] is DEPRECATED, use "true" or "false"`, dataName, dataValue)
			result = true
		case "disabled", "off":
			logger := GetLogger()
			logger.Warningf(`%s - [%s] is DEPRECATED, use "true" or "false"`, dataName, dataValue)
			result = false
		default:
			return false, err
		}
	}
	return result, nil
}

func GetPodPrefix(podName string) (prefix string, err error) {
	i := strings.LastIndex(podName, "-")
	if i == -1 {
		err = fmt.Errorf("incorrect podName format: '%s'", podName)
		return
	}
	i = strings.LastIndex(string([]rune(podName)[:i]), "-")
	if i == -1 {
		err = fmt.Errorf("incorrect podName format: '%s'", podName)
		return
	}
	prefix = string([]rune(podName)[:i])
	return
}

func EqualSliceStringsWithoutOrder(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	// In case order is different
	valuesInA := map[string]struct{}{}
	for _, value := range a {
		valuesInA[value] = struct{}{}
	}
	for _, value := range b {
		if _, ok := valuesInA[value]; !ok {
			return false
		}
	}
	return true
}

func CopyMap[K comparable, V any](mapArg map[K]V) map[K]V {
	mapCopy := make(map[K]V, len(mapArg))
	for k, v := range mapArg {
		mapCopy[k] = v
	}
	return mapCopy
}

func CopyMapOfMap[K comparable, K2 any, V map[K]K2](mapArg map[K]V) map[K]V {
	mapCopy := make(map[K]V, len(mapArg))
	for k, v := range mapArg {
		mapCopy[k] = CopyMap(v)
	}
	return mapCopy
}

type Pair[T, U any] struct {
	P1 T
	P2 U
}

// NewPair creates a new Pair with type inference.
func NewPair[T, U any](p1 T, p2 U) Pair[T, U] {
	return Pair[T, U]{P1: p1, P2: p2}
}

func PointerDefaultValueIfNil[T any](arg *T) T {
	if arg == nil {
		var a T
		return a
	}
	return *arg
}

// EqualSliceByIDFunc checks equality of two slices. No duplication check.
func EqualSliceByIDFunc[T any](a, b []T, id func(x T) string) bool {
	if len(a) != len(b) {
		return false
	}
	// In case order is different
	m := map[string]struct{}{}
	for _, value := range a {
		m[id(value)] = struct{}{}
	}
	for _, value := range b {
		if _, ok := m[id(value)]; !ok {
			return false
		}
	}
	return true
}
