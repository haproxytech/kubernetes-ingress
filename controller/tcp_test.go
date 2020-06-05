package controller

import "testing"

func Test_tcpOptions(t *testing.T) {
	type args struct {
		raw string
	}
	tests := []struct {
		name            string
		args            args
		wantNs          string
		wantName        string
		wantPort        string
		wantSslOption   string
		wantProxyOption string
		wantErr         bool
	}{
		{"proxy_v1", args{raw: "my-namespace/my-service:2222::proxy"}, "my-namespace", "my-service", "2222", "", "proxy", false},
		{"tls", args{raw: "my-namespace/my-service:2222:tls"}, "my-namespace", "my-service", "2222", "tls", "", false},
		{"error", args{raw: "error"}, "", "", "", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotNs, gotName, gotPort, gotSslOption, gotProxyOption, err := tcpOptions(tt.args.raw)
			if (err != nil) != tt.wantErr {
				t.Errorf("tcpOptions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotNs != tt.wantNs {
				t.Errorf("tcpOptions() gotNs = %v, want %v", gotNs, tt.wantNs)
			}
			if gotName != tt.wantName {
				t.Errorf("tcpOptions() gotName = %v, want %v", gotName, tt.wantName)
			}
			if gotPort != tt.wantPort {
				t.Errorf("tcpOptions() gotPort = %v, want %v", gotPort, tt.wantPort)
			}
			if gotSslOption != tt.wantSslOption {
				t.Errorf("tcpOptions() gotSslOption = %v, want %v", gotSslOption, tt.wantSslOption)
			}
			if gotProxyOption != tt.wantProxyOption {
				t.Errorf("tcpOptions() gotProxyOption = %v, want %v", gotProxyOption, tt.wantProxyOption)
			}
		})
	}
}
