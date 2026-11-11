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

package ingress

import (
	"bytes"
	"fmt"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/rules"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
)

func TestCredentialError(t *testing.T) {
	tests := []struct {
		name    string
		user    string
		pwd     string
		wantErr bool
	}{
		{name: "valid", user: "alice", pwd: "wonderland"},
		{name: "empty password", user: "alice", wantErr: true},
		{name: "backslash in password", user: "alice", pwd: `back\slash`, wantErr: true},
		{name: "backslash in user name", user: `ali\ce`, pwd: "wonderland", wantErr: true},
		{name: "single quote in password", user: "alice", pwd: "pass'word", wantErr: true},
		{name: "single quote in user name", user: "ali'ce", pwd: "wonderland", wantErr: true},
		{name: "dollar in password", user: "alice", pwd: "$2a$hash"},
		{name: "comment", user: "alice", pwd: "wonder#land", wantErr: true},
		{name: "newline", user: "alice", pwd: "wonder\nland", wantErr: true},
		{name: "carriage return", user: "alice", pwd: "wonder\rland", wantErr: true},
		{name: "tab", user: "alice", pwd: "wonder\tland", wantErr: true},
		{name: "space", user: "alice", pwd: "wonder land", wantErr: true},
		{name: "quote", user: "alice", pwd: `wonder"land`, wantErr: true},
		{name: "user name", user: "ali\nce", pwd: "wonderland", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := credentialError(tt.user, []byte(tt.pwd))
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func newAuthTestReqs(t *testing.T, ingress *store.Ingress, secret *store.Secret) (*ReqAuth, *rules.List) {
	t.Helper()
	list := &rules.List{}
	reqAuth := NewReqAuth(list, ingress)
	annotations := map[string]string{"auth-type": "basic-auth", "auth-secret": "creds"}
	k := store.K8s{Namespaces: map[string]*store.Namespace{
		"ns": {Secret: map[string]*store.Secret{"creds": secret}},
	}}
	for _, name := range []string{"auth-type", "auth-secret"} {
		require.NoError(t, reqAuth.NewAnnotation(name).Process(k, annotations))
	}
	return reqAuth, list
}

func TestAuthSecretKeepsValidUsersOnly(t *testing.T) {
	secret := &store.Secret{Status: store.ADDED, Data: map[string][]byte{
		"alice": []byte("wonderland"),
		"bob":   []byte("bad#password"),
		"carol": []byte("line\nbreak"),
		"dave":  []byte("with space"),
		"eve":   []byte(`quo"te`),
		"frank": []byte(`back\slash`),
		"grace": []byte("trailing\n"),
		"heidi": []byte("\n"),
	}}
	ingress := &store.Ingress{IngressCore: store.IngressCore{Namespace: "ns", Name: "ing"}}
	reqAuth, list := newAuthTestReqs(t, ingress, secret)

	require.NotNil(t, reqAuth.authRule)
	// The rule stays, so the invalid users cannot leave the route unauthenticated.
	assert.Len(t, *list, 1)
	assert.Equal(t, map[string][]byte{
		"alice": []byte("wonderland"),
		"grace": []byte("trailing"),
	}, reqAuth.authRule.Credentials)
}

func TestAuthSecretAllUsersInvalid(t *testing.T) {
	secret := &store.Secret{Status: store.ADDED, Data: map[string][]byte{
		"bob": []byte(`bad\password`),
	}}
	ingress := &store.Ingress{IngressCore: store.IngressCore{Namespace: "ns", Name: "ing"}}
	reqAuth, list := newAuthTestReqs(t, ingress, secret)

	require.NotNil(t, reqAuth.authRule)
	assert.Len(t, *list, 1)
	assert.Empty(t, reqAuth.authRule.Credentials)
}

func TestAuthSecretDoesNotLogPassword(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	t.Cleanup(func() { log.SetOutput(previous) })

	password := `secret\password`
	secret := &store.Secret{Status: store.ADDED, Data: map[string][]byte{"alice": []byte(password)}}
	ingress := &store.Ingress{IngressCore: store.IngressCore{Namespace: "ns", Name: "ing"}}
	newAuthTestReqs(t, ingress, secret)

	require.Contains(t, output.String(), "auth-secret: ignoring user")
	require.NotContains(t, output.String(), password)
}

func TestAuthRealm(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    string
		wantErr bool
	}{
		{name: "spaces", value: "Private Content", want: "Private-Content"},
		{name: "double quote", value: `realm" if TRUE`, want: "Protected-Content", wantErr: true},
		{name: "single quote", value: `realm' if TRUE`, want: "Protected-Content", wantErr: true},
		{name: "backslash", value: `realm\`, want: "Protected-Content", wantErr: true},
		{name: "open brace", value: `realm{`, want: "Protected-Content", wantErr: true},
		{name: "close brace", value: `realm}`, want: "Protected-Content", wantErr: true},
		{name: "environment", value: `realm$SECRET`, want: "Protected-Content", wantErr: true},
		{name: "comment", value: "realm#comment", want: "Protected-Content"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var list rules.List
			reqAuth := NewReqAuth(&list, nil)
			require.NoError(t, reqAuth.NewAnnotation("auth-type").Process(store.K8s{}, map[string]string{"auth-type": "basic-auth"}))
			err := reqAuth.NewAnnotation("auth-realm").Process(store.K8s{}, map[string]string{"auth-realm": tt.value})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Len(t, list, 1)
			require.Equal(t, tt.want, reqAuth.authRule.AuthRealm)
		})
	}
	for _, control := range []string{"\t", "\x00", "\x7f", "\r", "\n"} {
		t.Run(fmt.Sprintf("control_%q", control), func(t *testing.T) {
			var list rules.List
			reqAuth := NewReqAuth(&list, nil)
			require.NoError(t, reqAuth.NewAnnotation("auth-type").Process(store.K8s{}, map[string]string{"auth-type": "basic-auth"}))
			require.NoError(t, reqAuth.NewAnnotation("auth-realm").Process(store.K8s{}, map[string]string{"auth-realm": "realm" + control + "if TRUE"}))
			require.Len(t, list, 1)
			require.Equal(t, "Protected-Content", reqAuth.authRule.AuthRealm)
		})
	}
}

func TestAuthSecretWithoutNamespace(t *testing.T) {
	secret := &store.Secret{Status: store.ADDED, Data: map[string][]byte{"alice": []byte("wonderland")}}
	list := &rules.List{}
	reqAuth := NewReqAuth(list, nil)
	k := store.K8s{Namespaces: map[string]*store.Namespace{
		"ns": {Secret: map[string]*store.Secret{"creds": secret}},
	}}
	annotations := map[string]string{"auth-type": "basic-auth", "auth-secret": "creds"}

	require.NoError(t, reqAuth.NewAnnotation("auth-type").Process(k, annotations))
	assert.Error(t, reqAuth.NewAnnotation("auth-secret").Process(k, annotations))
	assert.Len(t, *list, 1)
	assert.Empty(t, reqAuth.authRule.Credentials)
}
