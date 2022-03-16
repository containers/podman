package kube

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"runtime"
	"strconv"
	"testing"

	"github.com/containers/common/pkg/secrets"
	v1 "github.com/containers/podman/v4/pkg/k8s.io/api/core/v1"
	"github.com/containers/podman/v4/pkg/k8s.io/apimachinery/pkg/api/resource"
	v12 "github.com/containers/podman/v4/pkg/k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/docker/docker/pkg/system"
	"github.com/stretchr/testify/assert"
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
	stringNumCPUs := strconv.Itoa(runtime.NumCPU())

	mi, err := system.ReadMemInfo()
	assert.Nil(t, err)
	stringMemTotal := strconv.FormatInt(mi.MemTotal, 10)

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
			nilString,
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
			nilString,
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
			nilString,
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
			nilString,
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
			nilString,
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
			nilString,
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
			nilString,
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
			nilString,
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
			nilString,
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
			nilString,
		},
		{
			"FieldRefMetadataName",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
			CtrSpecGenOptions{
				PodName: "test",
			},
			true,
			"test",
		},
		{
			"FieldRefMetadataUID",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.uid",
					},
				},
			},
			CtrSpecGenOptions{
				PodID: "ec71ff37c67b688598c0008187ab0960dc34e1dfdcbf3a74e3d778bafcfe0977",
			},
			true,
			"ec71ff37c67b688598c0008187ab0960dc34e1dfdcbf3a74e3d778bafcfe0977",
		},
		{
			"FieldRefMetadataLabelsExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels['label']",
					},
				},
			},
			CtrSpecGenOptions{
				Labels: map[string]string{"label": "label"},
			},
			true,
			"label",
		},
		{
			"FieldRefMetadataLabelsEmpty",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels['label']",
					},
				},
			},
			CtrSpecGenOptions{
				Labels: map[string]string{"label": ""},
			},
			true,
			"",
		},
		{
			"FieldRefMetadataLabelsNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.labels['label']",
					},
				},
			},
			CtrSpecGenOptions{},
			true,
			"",
		},
		{
			"FieldRefMetadataAnnotationsExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation']",
					},
				},
			},
			CtrSpecGenOptions{
				Annotations: map[string]string{"annotation": "annotation"},
			},
			true,
			"annotation",
		},
		{
			"FieldRefMetadataAnnotationsEmpty",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation']",
					},
				},
			},
			CtrSpecGenOptions{
				Annotations: map[string]string{"annotation": ""},
			},
			true,
			"",
		},
		{
			"FieldRefMetadataAnnotationsNotExist",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation']",
					},
				},
			},
			CtrSpecGenOptions{},
			true,
			"",
		},
		{
			"FieldRefInvalid1",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.annotations['annotation]",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"FieldRefInvalid2",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.dummy['annotation']",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"FieldRefNotSupported",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					FieldRef: &v1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefNotSupported",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.dummy",
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefMemoryDivisorNotValid",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
						Divisor:  resource.MustParse("2M"),
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefCpuDivisorNotValid",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.cpu",
						Divisor:  resource.MustParse("2m"),
					},
				},
			},
			CtrSpecGenOptions{},
			false,
			nilString,
		},
		{
			"ResourceFieldRefNoDivisor",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
					},
				},
			},
			CtrSpecGenOptions{
				Container: container,
			},
			true,
			memoryString,
		},
		{
			"ResourceFieldRefMemoryDivisor",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
						Divisor:  resource.MustParse("1Mi"),
					},
				},
			},
			CtrSpecGenOptions{
				Container: container,
			},
			true,
			strconv.Itoa(int(math.Ceil(float64(memoryInt) / 1024 / 1024))),
		},
		{
			"ResourceFieldRefCpuDivisor",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "requests.cpu",
						Divisor:  resource.MustParse("1m"),
					},
				},
			},
			CtrSpecGenOptions{
				Container: container,
			},
			true,
			strconv.Itoa(int(float64(cpuInt) / 0.001)),
		},
		{
			"ResourceFieldRefNoLimitMemory",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.memory",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringMemTotal,
		},
		{
			"ResourceFieldRefNoRequestMemory",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "requests.memory",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringMemTotal,
		},
		{
			"ResourceFieldRefNoLimitCPU",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "limits.cpu",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringNumCPUs,
		},
		{
			"ResourceFieldRefNoRequestCPU",
			v1.EnvVar{
				Name: "FOO",
				ValueFrom: &v1.EnvVarSource{
					ResourceFieldRef: &v1.ResourceFieldSelector{
						Resource: "requests.cpu",
					},
				},
			},
			CtrSpecGenOptions{
				Container: v1.Container{
					Name: "test",
				},
			},
			true,
			stringNumCPUs,
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			result, err := envVarValue(test.envVar, &test.options)
			assert.Equal(t, err == nil, test.succeed)
			if test.expected == nilString {
				assert.Nil(t, result)
			} else {
				fmt.Println(*result, test.expected)
				assert.Equal(t, &(test.expected), result)
			}
		})
	}
}

var (
	nilString     = "<nil>"
	configMapList = []v1.ConfigMap{
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

	optional = true

	k8sSecrets = []v1.Secret{
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

	cpuInt       = 4
	cpuString    = strconv.Itoa(cpuInt)
	memoryInt    = 30000000
	memoryString = strconv.Itoa(memoryInt)
	container    = v1.Container{
		Name: "test",
		Resources: v1.ResourceRequirements{
			Limits: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(cpuString),
				v1.ResourceMemory: resource.MustParse(memoryString),
			},
			Requests: v1.ResourceList{
				v1.ResourceCPU:    resource.MustParse(cpuString),
				v1.ResourceMemory: resource.MustParse(memoryString),
			},
		},
	}
)
