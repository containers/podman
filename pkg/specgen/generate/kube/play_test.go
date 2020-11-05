package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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
