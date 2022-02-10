package piecestorage

import (
	"fmt"
	"github.com/filecoin-project/venus-market/config"
	"reflect"
	"testing"
)

func TestParserProtocol(t *testing.T) {
	tests := []struct {
		name     string
		protocol string
		want     config.PieceStorage
		wantErr  bool
	}{
		{
			name:     "fs success",
			protocol: "fs:/mnt/test",
			want: config.PieceStorage{Fs: config.FsPieceStorage{
				Enable: true,
				Path:   "/mnt/test",
			}},
			wantErr: false,
		},
		{
			name:     "s3 success",
			protocol: "s3:ak:sk:t1@http://region1.s3.com/bucket1",
			want: config.PieceStorage{
				Fs: config.FsPieceStorage{},
				S3: config.S3PieceStorage{
					Enable:    true,
					EndPoint:  "http://region1.s3.com/bucket1",
					AccessKey: "ak",
					SecretKey: "sk",
					Token:     "t1",
				},
			},
			wantErr: false,
		},
		{
			name:     "s3 success",
			protocol: "s3:ak:sk:t1@http://region1.s3.com",
			wantErr:  true,
		},
		{
			name:     "s3 no token success",
			protocol: "s3:ak:sk@http://region1.s3.com/bucket1",
			want: config.PieceStorage{
				Fs: config.FsPieceStorage{},
				S3: config.S3PieceStorage{
					Enable:    true,
					EndPoint:  "http://region1.s3.com/bucket1",
					AccessKey: "ak",
					SecretKey: "sk",
					Token:     "",
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.PieceStorage{}
			err := ParserProtocol(tt.protocol, &cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParserProtocol() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			t1 := reflect.TypeOf(cfg)
			t2 := reflect.TypeOf(tt.want)
			fmt.Println(t1.String())
			fmt.Println(t2.String())
			if !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("ParserProtocol() got = %v, want %v", cfg, tt.want)
			}
		})
	}

}
