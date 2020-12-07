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

package haproxy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/utils"
	"github.com/haproxytech/models/v2"
)

type Server models.Server

func (s *Server) ResetSendProxy() {
	s.SendProxy = ""
	s.SendProxyV2 = ""
}

func (s *Server) UpdateSendProxy(value string) (err error) {
	switch strings.ToLower(value) {
	case "proxy":
		s.SendProxy = "enabled"
	case "proxy-v1":
		s.SendProxy = "enabled"
	case "proxy-v2":
		s.SendProxyV2 = "enabled"
	default:
		err = fmt.Errorf("%s is an unknown enum", value)
	}
	return
}

func (s *Server) UpdateCheck(value string) error {
	enabled, err := utils.GetBoolValue(value, "check")
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

func (s *Server) UpdateInter(value string) error {
	time, err := utils.ParseTime(value)
	if err != nil {
		return err
	}
	s.Inter = time
	return nil
}

func (s *Server) UpdateMaxconn(value string) error {
	maxconn, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return err
	}
	s.Maxconn = &maxconn
	return nil
}

func (s *Server) UpdateProto(value string) error {
	if value == "h2" {
		s.Proto = "h2"
		return nil
	}
	return fmt.Errorf("unkown proto %s", value)
}
