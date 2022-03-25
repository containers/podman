package qemu

import (
	"reflect"
	"testing"
)

func TestMachineFile_GetPath(t *testing.T) {
	path := "/var/tmp/podman/my.sock"
	sym := "/tmp/podman/my.sock"
	type fields struct {
		Path    string
		Symlink *string
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name:   "Original path",
			fields: fields{path, nil},
			want:   path,
		},
		{
			name: "Symlink over path",
			fields: fields{
				Path:    path,
				Symlink: &sym,
			},
			want: sym,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &MachineFile{
				Path:    tt.fields.Path,    //nolint: scopelint
				Symlink: tt.fields.Symlink, //nolint: scopelint
			}
			if got := m.GetPath(); got != tt.want { //nolint: scopelint
				t.Errorf("GetPath() = %v, want %v", got, tt.want) //nolint: scopelint
			}
		})
	}
}

func TestNewMachineFile(t *testing.T) {
	p := "/var/tmp/podman/my.sock"
	sym := "/tmp/podman/my.sock"
	empty := ""

	m := MachineFile{
		Path:    p,
		Symlink: nil,
	}
	type args struct {
		path    string
		symlink *string
	}
	tests := []struct {
		name    string
		args    args
		want    *MachineFile
		wantErr bool
	}{
		{
			name:    "Good",
			args:    args{path: p},
			want:    &m,
			wantErr: false,
		},
		{
			name:    "Good with Symlink",
			args:    args{p, &sym},
			want:    &MachineFile{p, &sym},
			wantErr: false,
		},
		{
			name:    "Bad path name",
			args:    args{empty, nil},
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Bad symlink name",
			args:    args{p, &empty},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMachineFile(tt.args.path, tt.args.symlink) //nolint: scopelint
			if (err != nil) != tt.wantErr {                           //nolint: scopelint
				t.Errorf("NewMachineFile() error = %v, wantErr %v", err, tt.wantErr) //nolint: scopelint
				return
			}
			if !reflect.DeepEqual(got, tt.want) { //nolint: scopelint
				t.Errorf("NewMachineFile() got = %v, want %v", got, tt.want) //nolint: scopelint
			}
		})
	}
}
