package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestEnvVarsFrom(t *testing.T) {
	tests := []struct {
		name     string
		envFrom  v1.EnvFromSource
		options  CtrSpecGenOptions
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
			map[string]string{},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result := envVarsFrom(test.envFrom, &test.options)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestEnvVarValue(t *testing.T) {
	tests := []struct {
		name     string
		envVar   v1.EnvVar
		options  CtrSpecGenOptions
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
			"",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result := envVarValue(test.envVar, &test.options)
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
