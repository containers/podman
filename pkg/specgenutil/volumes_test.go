package specgenutil

import "testing"

func Test_validChownFlag(t *testing.T) {
	type args struct {
		flag string
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "U true",
			args: args{
				flag: "U=true",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "U true case does not matter",
			args: args{
				flag: "u=True",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "U is false",
			args: args{
				flag: "U=false",
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "chown should also work",
			args: args{
				flag: "chown=true",
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "garbage value should fail",
			args: args{
				flag: "U=foobar",
			},
			want:    false,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := validChownFlag(tt.args.flag)
			if (err != nil) != tt.wantErr {
				t.Errorf("validChownFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("validChownFlag() got = %v, want %v", got, tt.want)
			}
		})
	}
}
