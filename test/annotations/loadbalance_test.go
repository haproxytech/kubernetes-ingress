package annotations_test

import (
	"reflect"
	"testing"

	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/service"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

func TestGetParamsFromInput(t *testing.T) {
	type args struct {
		value string
	}
	tests := []struct {
		name    string
		args    args
		want    *models.Balance
		wantErr bool
	}{
		{
			name: "roundrobin",
			args: args{
				value: "roundrobin",
			},
			want: &models.Balance{
				Algorithm: utils.PtrString("roundrobin"),
			},
		},
		{
			name: "hdr",
			args: args{
				value: "hdr(User-Agent)",
			},
			want: &models.Balance{
				Algorithm: utils.PtrString("hdr"),
				HdrName:   "User-Agent",
			},
		},
		{
			name: "hdr-2",
			args: args{
				value: "hdr(Host) use_domain_only",
			},
			want: &models.Balance{
				Algorithm:        utils.PtrString("hdr"),
				HdrName:          "Host",
				HdrUseDomainOnly: true,
			},
		},
		{
			name: "random ok",
			args: args{
				value: "random(10)",
			},
			want: &models.Balance{
				Algorithm:   utils.PtrString("random"),
				RandomDraws: 10,
			},
		},
		{
			name: "random ko",
			args: args{
				value: "random(notok)",
			},
			want: &models.Balance{
				Algorithm: utils.PtrString("random"),
			},
			wantErr: true,
		},
		{
			name: "rdp cookie",
			args: args{
				value: "rdp-cookie(cookiename)",
			},
			want: &models.Balance{
				Algorithm:     utils.PtrString("rdp-cookie"),
				RdpCookieName: "cookiename",
			},
		},
		{
			name: "url_param",
			args: args{
				value: "url_param session_id check_post 64",
			},
			want: &models.Balance{
				Algorithm:         utils.PtrString("url_param"),
				URLParam:          "session_id",
				URLParamCheckPost: 64,
			},
		},
		{
			name: "uri",
			args: args{
				value: "uri len 2 depth 3",
			},
			want: &models.Balance{
				Algorithm: utils.PtrString("uri"),
				URILen:    2,
				URIDepth:  3,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetParamsFromInput(tt.args.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetParamsFromInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetParamsFromInput() = %v, want %v", got, tt.want)
			}
		})
	}
}
