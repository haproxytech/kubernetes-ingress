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

package main

import (
	"github.com/haproxytech/client-native/misc"
	"github.com/haproxytech/models"
	"strconv"
)

type Server models.Server

func (s *Server) updateCheck(data *StringW) error {
	enabled, err := GetBoolValue(data.Value, "check")
	if err != nil {
		return err
	}
	if enabled {
		s.Check = "enabled"
	} else {
		s.Check = "disabled"
	}
	return nil
}

func (s *Server) updateInter(data *StringW) error {
	time := misc.ParseTimeout(data.Value)
	s.Inter = time
	return nil
}

func (s *Server) updateMaxconn(data *StringW) error {
	maxconn, err := strconv.ParseInt(data.Value, 10, 64)
	if err != nil {
		return err
	}
	s.Maxconn = &maxconn
	return nil
}
