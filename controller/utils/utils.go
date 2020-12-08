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
	"hash/fnv"
	"math/rand"
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

var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

//RandomString returns random string of size n
func RandomString(n int) string {
	b := make([]rune, n)
	size := len(chars)
	for i := range b {
		b[i] = chars[rand.Intn(size)]
	}
	return string(b)
}

func Hash(input []byte) uint32 {
	h := fnv.New32a()
	_, _ = h.Write(input)
	return h.Sum32()
}

func PtrInt64(value int64) *int64 {
	return &value
}

//nolint deadcode
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
