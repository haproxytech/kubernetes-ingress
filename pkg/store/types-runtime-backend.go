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

package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/haproxytech/client-native/v6/misc"
)

// serverNameHashLen keeps names short in logs, stats and cookies while staying unique per backend.
const serverNameHashLen = 24

// ComputeServerName derives a stable server name from the endpoint address and port.
func (re RuntimeEndpoint) ComputeServerName() string {
	serverAddr := fmt.Sprintf("%s:%d", misc.SanitizeIPv6Address(re.Address), re.Port)
	return "s" + hashHex(serverAddr)[:serverNameHashLen]
}

func hashHex(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
