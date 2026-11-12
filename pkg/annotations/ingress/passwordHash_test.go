package ingress

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPasswordHashError(t *testing.T) {
	h22 := strings.Repeat("a", 22)
	h43 := strings.Repeat("a", 43)
	h53 := strings.Repeat("a", 53)
	h86 := strings.Repeat("a", 86)
	tests := []struct {
		name    string
		hash    string
		wantErr bool
	}{
		{name: "des", hash: "mskYxuygva2Ys"},
		{name: "md5", hash: "$1$oGq7nTAf$" + h22},
		{name: "sha256", hash: "$5$saltsalt$" + h43},
		{name: "sha512", hash: "$6$saltsalt$" + h86},
		{name: "sha512 rounds", hash: "$6$rounds=656000$saltsalt$" + h86},
		{name: "bcrypt a", hash: "$2a$10$" + h53},
		{name: "bcrypt b", hash: "$2b$04$" + h53},
		{name: "bcrypt y", hash: "$2y$12$" + h53},

		{name: "plain text", hash: "wonderland", wantErr: true},
		{name: "one character", hash: "x", wantErr: true},
		{name: "locked account", hash: "*", wantErr: true},
		{name: "des bad character", hash: "a:cdefghijklm", wantErr: true},
		{name: "extended des", hash: "_J9..rasmBnkabcdefgh", wantErr: true},
		{name: "unknown scheme", hash: "$9$salt$" + h43, wantErr: true},
		{name: "yescrypt", hash: "$y$j9T$saltsalt$" + h43, wantErr: true},
		{name: "argon2", hash: "$argon2id$v=19$m=65536,t=3,p=4$c2FsdA$aGFzaA", wantErr: true},
		{name: "md5 truncated", hash: "$1$salt$short", wantErr: true},
		{name: "md5 empty salt", hash: "$1$$" + h22, wantErr: true},
		{name: "sha512 truncated", hash: "$6$salt$short", wantErr: true},
		{name: "sha512 bad salt", hash: "$6$sa:lt$" + h86, wantErr: true},
		{name: "sha512 long salt", hash: "$6$" + strings.Repeat("s", 17) + "$" + h86, wantErr: true},
		{name: "rounds too low", hash: "$6$rounds=999$saltsalt$" + h86, wantErr: true},
		{name: "rounds leading zero", hash: "$6$rounds=01000$saltsalt$" + h86, wantErr: true},
		{name: "rounds stall the check", hash: "$6$rounds=999999999$saltsalt$" + h86, wantErr: true},
		{name: "rounds overflow", hash: "$6$rounds=99999999999999999999$saltsalt$" + h86, wantErr: true},
		{name: "rounds empty", hash: "$6$rounds=$saltsalt$" + h86, wantErr: true},
		{name: "rounds negative", hash: "$6$rounds=-5000$saltsalt$" + h86, wantErr: true},
		{name: "bcrypt cost too low", hash: "$2y$03$" + h53, wantErr: true},
		{name: "bcrypt cost stalls the check", hash: "$2y$31$" + h53, wantErr: true},
		{name: "bcrypt cost not a number", hash: "$2y$1x$" + h53, wantErr: true},
		{name: "bcrypt truncated", hash: "$2y$05$abc", wantErr: true},
		{name: "bcrypt unknown variant", hash: "$2$05$" + h53, wantErr: true},
		{name: "lone dollar", hash: "$", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := passwordHashError(tt.hash)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}
