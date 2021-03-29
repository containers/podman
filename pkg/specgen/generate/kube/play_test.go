package kube

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/containers/common/pkg/secrets"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createSecrets(t *testing.T, d string) *secrets.SecretsManager {
	secretsManager, err := secrets.NewManager(d)
	assert.NoError(t, err)

	driver := "file"
	driverOpts := map[string]string{
		"path": d,
	}

	for _, s := range k8sSecrets {
		data, err := json.Marshal(s.Data)
		assert.NoError(t, err)

		_, err = secretsManager.Store(s.ObjectMeta.Name, data, driver, driverOpts)
		assert.NoError(t, err)
	}

	return secretsManager
}

func TestEnvVarsFrom(t *testing.T) {
	d, err := ioutil.TempDir("", "secrets")
	assert.NoError(t, err)
	defer os.RemoveAll(d)
	secretsManager := createSecrets(t, d)

	tests := []struct {
		name     string
		envFrom  v1.EnvFromSource
		options  CtrSpecGenOptions
		succeed  bool
		expected map[string]string
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
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
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
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			false,
			nil,
		},
		{
			"OptionalConfigMapDoesNotExist",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
					Optional: &optional,
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
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
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			false,
			nil,
		},
		{
			"OptionalEmptyConfigMapList",
			v1.EnvFromSource{
				ConfigMapRef: &v1.ConfigMapEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
					Optional: &optional,
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			true,
			map[string]string{},
		},
		{
			"SecretExists",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "foo",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			map[string]string{
				"myvar": "foo",
			},
		},
		{
			"SecretDoesNotExist",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			false,
			nil,
		},
		{
			"OptionalSecretDoesNotExist",
			v1.EnvFromSource{
				SecretRef: &v1.SecretEnvSource{
					LocalObjectReference: v1.LocalObjectReference{
						Name: "doesnotexist",
					},
					Optional: &optional,
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			map[string]string{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result, err := envVarsFrom(test.envFrom, &test.options)
			assert.Equal(t, err == nil, test.succeed)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestEnvVarValue(t *testing.T) {
	d, err := ioutil.TempDir("", "secrets")
	assert.NoError(t, err)
	defer os.RemoveAll(d)
	secretsManager := createSecrets(t, d)

	tests := []struct {
		name     string
		envVar   v1.EnvVar
		options  CtrSpecGenOptions
		succeed  bool
		expected string
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
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
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
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			false,
			"",
		},
		{
			"OptionalContainerKeyDoesNotExistInConfigMap",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key:      "doesnotexist",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
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
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			false,
			"",
		},
		{
			"OptionalConfigMapDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key:      "myvar",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: configMapList,
			},
			true,
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
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			false,
			"",
		},
		{
			"OptionalEmptyConfigMapList",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ConfigMapKeyRef: &v1.ConfigMapKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key:      "myvar",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				ConfigMaps: []v1.ConfigMap{},
			},
			true,
			"",
		},
		{
			"SecretExists",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "myvar",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			"foo",
		},
		{
			"ContainerKeyDoesNotExistInSecret",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key: "doesnotexist",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			false,
			"",
		},
		{
			"OptionalContainerKeyDoesNotExistInSecret",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "foo",
						},
						Key:      "doesnotexist",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			"",
		},
		{
			"SecretDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key: "myvar",
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			false,
			"",
		},
		{
			"OptionalSecretDoesNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					SecretKeyRef: &v1.SecretKeySelector{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "doesnotexist",
						},
						Key:      "myvar",
						Optional: &optional,
					},
				},
			},
			CtrSpecGenOptions{
				SecretsManager: secretsManager,
			},
			true,
			"",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result, err := envVarValue(test.envVar, &test.options)
			assert.Equal(t, err == nil, test.succeed)
			assert.Equal(t, test.expected, result)
		})
	}
}

var configMapList = []v1.ConfigMap{
	{
		TypeMeta: v12.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: v12.ObjectMeta{
			Name: "bar",
		},
		Data: map[string]string{
			"myvar": "bar",
		},
	},
	{
		TypeMeta: v12.TypeMeta{
			Kind: "ConfigMap",
		},
		ObjectMeta: v12.ObjectMeta{
			Name: "foo",
		},
		Data: map[string]string{
			"myvar": "foo",
		},
	},
}

var optional = true

var k8sSecrets = []v1.Secret{
	{
		TypeMeta: v12.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: v12.ObjectMeta{
			Name: "bar",
		},
		Data: map[string][]byte{
			"myvar": []byte("bar"),
		},
	},
	{
		TypeMeta: v12.TypeMeta{
			Kind: "Secret",
		},
		ObjectMeta: v12.ObjectMeta{
			Name: "foo",
		},
		Data: map[string][]byte{
			"myvar": []byte("foo"),
		},
	},
}
