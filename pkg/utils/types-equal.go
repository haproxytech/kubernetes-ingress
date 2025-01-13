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

import "github.com/haproxytech/client-native/v6/models"

type Equalizer[T any] interface {
	Equal(t T, opt ...models.Options) bool
}

func EqualSlice[T Equalizer[T]](sliceA, sliceB []T, opt ...models.Options) bool {
	if len(sliceA) != len(sliceB) {
		return false
	}
	for i, value := range sliceA {
		if !value.Equal(sliceB[i], opt...) {
			return false
		}
	}
	return true
}

func EqualSliceComparable[T comparable](sliceA, sliceB []T) bool {
	if len(sliceA) != len(sliceB) {
		return false
	}
	for i, value := range sliceA {
		if value != sliceB[i] {
			return false
		}
	}
	return true
}

type Literal interface {
	~int | ~uint | ~float32 | ~float64 | ~complex64 | ~complex128 | ~int32 | ~int64 | ~string | ~bool
}

func EqualPointers[P Literal](a, b *P) bool {
	return (a == nil && b == nil) || (a != nil && b != nil) && *a == *b
}

func EqualPointersEqualizer[P Equalizer[P]](a, b *P, opt ...models.Options) bool {
	return (a == nil && b == nil) || ((a != nil && b != nil) && (*a).Equal(*b, opt...))
}

func EqualMap[T, V Literal](mapA, mapB map[T]V) bool {
	if mapA == nil && mapB == nil {
		return true
	}
	if mapA == nil || mapB == nil {
		return false
	}
	if len(mapA) != len(mapB) {
		return false
	}
	for k, v := range mapA {
		if mapB[k] != v {
			return false
		}
	}
	return true
}
