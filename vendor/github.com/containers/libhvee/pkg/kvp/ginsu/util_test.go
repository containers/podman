package ginsu

import (
	"bytes"
	"reflect"
	"testing"
)

func TestDice(t *testing.T) {
	type args struct {
		k *bytes.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Dice(tt.args.k)
			if (err != nil) != tt.wantErr {
				t.Errorf("Dice() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Dice() got = %v, want %v", got, tt.want)
			}
		})
	}
}
