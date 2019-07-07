package systemdgen

import (
	"testing"
)

func TestValidateRestartPolicy(t *testing.T) {
	type args struct {
		restart string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{"good-on", args{restart: "no"}, false},
		{"good-on-success", args{restart: "on-success"}, false},
		{"good-on-failure", args{restart: "on-failure"}, false},
		{"good-on-abnormal", args{restart: "on-abnormal"}, false},
		{"good-on-watchdog", args{restart: "on-watchdog"}, false},
		{"good-on-abort", args{restart: "on-abort"}, false},
		{"good-always", args{restart: "always"}, false},
		{"fail", args{restart: "foobar"}, true},
		{"failblank", args{restart: ""}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ValidateRestartPolicy(tt.args.restart); (err != nil) != tt.wantErr {
				t.Errorf("ValidateRestartPolicy() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateSystemdUnitAsString(t *testing.T) {
	goodID := `[Unit]
Description=639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401 Podman Container
[Service]
Restart=always
ExecStart=/usr/bin/podman start 639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401
ExecStop=/usr/bin/podman stop -t 10 639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401
KillMode=none
Type=forking
PIDFile=/var/run/containers/storage/overlay-containers/639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401/userdata/conmon.pid
[Install]
WantedBy=multi-user.target`

	goodName := `[Unit]
Description=foobar Podman Container
[Service]
Restart=always
ExecStart=/usr/bin/podman start foobar
ExecStop=/usr/bin/podman stop -t 10 foobar
KillMode=none
Type=forking
PIDFile=/var/run/containers/storage/overlay-containers/639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401/userdata/conmon.pid
[Install]
WantedBy=multi-user.target`

	type args struct {
		exe         string
		name        string
		cid         string
		restart     string
		pidFile     string
		stopTimeout int
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{

		{"good with id",
			args{
				"/usr/bin/podman",
				"639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401",
				"639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401",
				"always",
				"/var/run/containers/storage/overlay-containers/639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401/userdata/conmon.pid",
				10,
			},
			goodID,
			false,
		},
		{"good with name",
			args{
				"/usr/bin/podman",
				"foobar",
				"639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401",
				"always",
				"/var/run/containers/storage/overlay-containers/639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401/userdata/conmon.pid",
				10,
			},
			goodName,
			false,
		},
		{"bad restart policy",
			args{
				"/usr/bin/podman",
				"639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401",
				"639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401",
				"never",
				"/var/run/containers/storage/overlay-containers/639c53578af4d84b8800b4635fa4e680ee80fd67e0e6a2d4eea48d1e3230f401/userdata/conmon.pid",
				10,
			},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createSystemdUnitAsString(tt.args.exe, tt.args.name, tt.args.cid, tt.args.restart, tt.args.pidFile, tt.args.stopTimeout)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateSystemdUnitAsString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("CreateSystemdUnitAsString() = %v, want %v", got, tt.want)
			}
		})
	}
}
