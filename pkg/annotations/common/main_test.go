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

package common

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsSet(t *testing.T) {
	annotations := map[string]string{"check": "false"}
	require.True(t, IsSet("check", annotations), "present annotation is set")
	require.True(t, IsSet("check", map[string]string{"check": ""}), "an explicitly empty value is still a use")
	require.False(t, IsSet("load-balance", annotations), "absent annotation is not set")
	require.False(t, IsSet("check"), "no maps means nothing is set")
}

func TestHasCommentChar(t *testing.T) {
	require.True(t, HasCommentChar("bar#baz"), "mid-token '#' starts a comment")
	require.True(t, HasCommentChar("#"))
	require.False(t, HasCommentChar("bar baz"))
}

func TestHasControlChar(t *testing.T) {
	require.True(t, HasControlChar("bar\r"))
	require.True(t, HasControlChar("bar\tbaz"))
	require.True(t, HasControlChar("bar\x7f"))
	require.False(t, HasControlChar("bar baz"))
}

func TestGetValueCommentChar(t *testing.T) {
	annotations := map[string]string{
		"set-host":           "evil.local #",
		"auth-realm":         "realm #",
		"check":              "true #",
		"log-format":         "%ci # %cp",
		"path-rewrite":       "(.*) /foo#bar",
		"request-set-header": `X-Foo "bar#baz"`,
	}
	require.Equal(t, "", GetValue("set-host", annotations), "'#' would comment out the scope guard")
	require.Equal(t, "Protected Content", GetValue("auth-realm", annotations), "'#' falls back to the default realm")
	require.Equal(t, "true", GetValue("check", annotations), "'#' falls back to the default")
	require.Equal(t, "", GetValue("log-format", annotations), "'#' is rejected in single-line annotations")
	require.Equal(t, "", GetValue("path-rewrite", annotations), "'#' is rejected in multiline annotations too")
	require.Equal(t, "", GetValue("request-set-header", annotations), "'#' is rejected even inside quotes")
}

func TestGetValueMultiline(t *testing.T) {
	annotations := map[string]string{
		"check":            "true\nhttp-request deny",
		"set-host":         "evil.local\r\nhttp-request deny",
		"path-rewrite":     "/foo/(.*) /\\1\n/bar/(.*) /baz/\\1",
		"syslog-server":    "address:127.0.0.1, port:514\naddress:127.0.0.2, port:514",
		"load-balance":     "roundrobin",
		"request-redirect": "example.com\n",
		"request-capture":  "hdr(Host)\nhdr(User-Agent)",
	}
	require.Equal(t, "true", GetValue("check", annotations), "multiline value falls back to the default")
	require.Equal(t, "", GetValue("set-host", annotations), "CR is discarded too")
	require.Equal(t, "", GetValue("request-redirect", annotations), "trailing LF is discarded")
	require.Equal(t, "roundrobin", GetValue("load-balance", annotations), "single-line value is kept")
	require.Equal(t, annotations["path-rewrite"], GetValue("path-rewrite", annotations), "path-rewrite takes multiple lines")
	require.Equal(t, annotations["request-capture"], GetValue("request-capture", annotations), "request-capture takes multiple lines")
	require.Equal(t, annotations["syslog-server"], GetValue("syslog-server", annotations), "syslog-server takes multiple lines")
}

func TestGetValueRejectsSingleLineControls(t *testing.T) {
	for control := rune(0); control <= 0x7f; control++ {
		if control >= 0x20 && control != 0x7f {
			continue
		}
		values := map[string]string{
			"set-host":   "host" + string(control) + "if TRUE",
			"auth-realm": "realm" + string(control) + "if TRUE",
		}
		require.Empty(t, GetValue("set-host", values), "control %#x", control)
		require.Equal(t, DefaultValues["auth-realm"], GetValue("auth-realm", values), "control %#x", control)
	}
}
