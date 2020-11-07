package abi

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"reflect"
	"testing"

	"github.com/containers/podman/v2/libpod"
	"github.com/containers/podman/v2/pkg/domain/entities"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var configMapList = []v1.ConfigMap{
	{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "bar",
		},
		Data: map[string]string{
			"myvar": "bar",
		},
	},
	{
		TypeMeta: metav1.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Data: map[string]string{
			"myvar": "foo",
		},
	},
}

func TestReadConfigMapFromFile(t *testing.T) {
	tests := []struct {
		name             string
		configMapContent string
		expectError      bool
		expectedErrorMsg string
		expected         v1.ConfigMap
	}{
		{
			"ValidConfigMap",
			`
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myvar: foo
`,
			false,
			"",
			v1.ConfigMap{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ConfigMap",
					APIVersion: "v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "foo",
				},
				Data: map[string]string{
					"myvar": "foo",
				},
			},
		},
		{
			"InvalidYAML",
			`
Invalid YAML
apiVersion: v1
kind: ConfigMap
metadata:
  name: foo
data:
  myvar: foo
`,
			true,
			"unable to read YAML as Kube ConfigMap",
			v1.ConfigMap{},
		},
		{
			"InvalidKind",
			`
apiVersion: v1
kind: InvalidKind
metadata:
  name: foo
data:
  myvar: foo
`,
			true,
			"invalid YAML kind",
			v1.ConfigMap{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			buf := bytes.NewBufferString(test.configMapContent)
			cm, err := readConfigMapFromFile(buf)

			if test.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedErrorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expected, cm)
			}
		})
	}
}

func TestEnvVarsFromConfigMap(t *testing.T) {
	tests := []struct {
		name          string
		envFrom       v1.EnvFromSource
		configMapList []v1.ConfigMap
		expected      map[string]string
	}{
		{
			"ConfigMapExists",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
				},
			},
			configMapList,
			map[string]string{
				"myvar": "foo",
			},
		},
		{
			"ConfigMapDoesNotExist",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
				},
			},
			configMapList,
			map[string]string{},
		},
		{
			"EmptyConfigMapList",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
				},
			},
			[]v1.ConfigMap{},
			map[string]string{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result := envVarsFromConfigMap(test.envFrom, test.configMapList)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestEnvVarValue(t *testing.T) {
	tests := []struct {
		name          string
		envVar        v1.EnvVar
		configMapList []v1.ConfigMap
		expected      string
	}{
		{
			"ConfigMapExists",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "myvar",
					},
				},
			},
			configMapList,
			"foo",
		},
		{
			"ContainerKeyDoesNotExistInConfigMap",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "doesnotexist",
					},
				},
			},
			configMapList,
			"",
		},
		{
			"ConfigMapDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key: "myvar",
					},
				},
			},
			configMapList,
			"",
		},
		{
			"EmptyConfigMapList",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "myvar",
					},
				},
			},
			[]v1.ConfigMap{},
			"",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result := envVarValue(test.envVar, test.configMapList)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestContainerEngine_PlayKube(t *testing.T) {
	type fields struct {
		Libpod *libpod.Runtime
	}
	type args struct {
		ctx     context.Context
		path    string
		options entities.PlayKubeOptions
	}
	type test struct {
		name    string
		fields  fields
		args    args
		want    *entities.PlayKubeReport
		wantErr bool
	}

	runtime, err := libpod.NewRuntime(context.TODO())
	if err != nil {
		t.Error(err)
	}

	contents := [][]string{
		{"PersistentVolume", `
apiVersion: v1
kind: PersistentVolume
metadata:
  name: foo-pv
spec:
  storageClassName: ""
  claimRef:
    name: foo-pvc
    namespace: foo
`},
	}

	files := []*os.File{}
	tests := []test{}
	for _, content := range contents {
		tmpfile, err := ioutil.TempFile("", "playfile")
		if err != nil {
			t.Error(err)
		}
		files = append(files, tmpfile)

		if _, err := tmpfile.Write([]byte(content[1])); err != nil {
			t.Error(err)
		}

		tests = append(tests, test{
			name:   content[0],
			fields: fields{Libpod: runtime},
			args: args{
				ctx:     context.TODO(),
				options: entities.PlayKubeOptions{},
				path:    tmpfile.Name(),
			},
			want: &entities.PlayKubeReport{
				Volumes: []entities.PlayKubeVolume{
					{Name: "foo-pvc"},
				},
			},
			wantErr: false,
		})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ic := &ContainerEngine{
				Libpod: tt.fields.Libpod,
			}
			got, err := ic.PlayKube(tt.args.ctx, tt.args.path, tt.args.options)
			if (err != nil) != tt.wantErr {
				t.Errorf("ContainerEngine.PlayKube() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ContainerEngine.PlayKube() = %v, want %v", got, tt.want)
			}
		})
	}
}
