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

//go:build e2e_sequential

package endpoints

import "io"

func (suite *EndpointsSuite) Test_TCP_Reach() {
	counter := map[string]int{}
	for i := 0; i < 4; i++ {
		func() {
			res, cls, err := suite.client.Do()
			if err != nil {
				suite.Require().NoError(err)
				return
			}
			defer cls()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				suite.Error(err)
				return
			}
			counter[string(body)]++
		}()
	}
	for _, v := range counter {
		suite.Equal(4, v)
	}
}
