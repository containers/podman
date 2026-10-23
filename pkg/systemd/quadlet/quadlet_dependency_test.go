package quadlet

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.podman.io/podman/v6/pkg/systemd/parser"
)

func TestTranslateUnitDependencies(t *testing.T) {
	tests := []struct {
		name               string
		unitFilename       string
		dependency         string
		unitsInfoMap       map[string]*UnitInfo
		wantError          bool
		errorContains      string
		expectedDependency string
	}{
		{
			name:         "template dependency with instance propagation (issue #29819)",
			unitFilename: "test2@instance1.container",
			dependency:   "test1@.container",
			unitsInfoMap: map[string]*UnitInfo{
				"test1@instance1.container": {
					ServiceName:  "test1@instance1",
					ResourceName: "test1-instance1",
				},
			},
			wantError:          false,
			expectedDependency: "test1@instance1.service",
		},
		{
			name:         "template dependency with typo",
			unitFilename: "test2@instance1.container",
			dependency:   "testt1@.container",
			unitsInfoMap: map[string]*UnitInfo{
				"test1@instance1.container": {
					ServiceName:  "test1@instance1",
					ResourceName: "test1-instance1",
				},
			},
			wantError:     true,
			errorContains: "unable to translate dependency",
		},
		{
			name:         "non-template dependency with typo",
			unitFilename: "test2.container",
			dependency:   "testt1.container",
			unitsInfoMap: map[string]*UnitInfo{
				"test1.container": {
					ServiceName:  "test1",
					ResourceName: "test1",
				},
			},
			wantError:     true,
			errorContains: "unable to translate dependency",
		},
		{
			name:         "valid non-template dependency",
			unitFilename: "test2.container",
			dependency:   "test1.container",
			unitsInfoMap: map[string]*UnitInfo{
				"test1.container": {
					ServiceName:  "test1",
					ResourceName: "test1",
				},
			},
			wantError:          false,
			expectedDependency: "test1.service",
		},
		{
			name:         "non-template unit referring to template in map",
			unitFilename: "test2.container",
			dependency:   "test1@.container",
			unitsInfoMap: map[string]*UnitInfo{
				"test1@.container": {
					ServiceName:  "test1@",
					ResourceName: "test1-%i",
				},
			},
			wantError:          false,
			expectedDependency: "test1@.service",
		},
		{
			name:          "non-template unit referring to template not in map",
			unitFilename:  "test2.container",
			dependency:    "test1@.container",
			unitsInfoMap:  map[string]*UnitInfo{},
			wantError:     true,
			errorContains: "unable to translate dependency",
		},
		{
			name:         "explicit instantiated dependency (no propagation)",
			unitFilename: "test2@instance1.container",
			dependency:   "test1@instance2.container",
			unitsInfoMap: map[string]*UnitInfo{
				"test1@instance2.container": {
					ServiceName:  "test1@instance2",
					ResourceName: "test1-instance2",
				},
			},
			wantError:          false,
			expectedDependency: "test1@instance2.service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unitFile := parser.NewUnitFile()
			unitFile.Filename = tt.unitFilename
			unitFile.Add("Unit", "Conflicts", tt.dependency)

			err := translateUnitDependencies(unitFile, tt.unitsInfoMap)

			if tt.wantError {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
				deps := unitFile.LookupAllStrv("Unit", "Conflicts")
				require.Len(t, deps, 1)
				require.Equal(t, tt.expectedDependency, deps[0])
			}
		})
	}
}

func TestExtractInstanceName(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		expected string
	}{
		{
			name:     "template with instance",
			filename: "test@instance1.container",
			expected: "instance1",
		},
		{
			name:     "template with multi-char instance",
			filename: "test@my-instance.container",
			expected: "my-instance",
		},
		{
			name:     "uninstantiated template",
			filename: "test@.container",
			expected: "",
		},
		{
			name:     "non-template",
			filename: "test.container",
			expected: "",
		},
		{
			name:     "instance with @ character (systemd allows this)",
			filename: "test@instance@1.container",
			expected: "instance@1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractInstanceName(tt.filename)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGenerateInstanceName(t *testing.T) {
	tests := []struct {
		name         string
		templateName string
		instanceName string
		expected     string
	}{
		{
			name:         "template with instance",
			templateName: "test@.container",
			instanceName: "instance1",
			expected:     "test@instance1.container",
		},
		{
			name:         "template without instance",
			templateName: "test@.container",
			instanceName: "",
			expected:     "test@.container",
		},
		{
			name:         "template with multi-char instance",
			templateName: "test@.container",
			instanceName: "my-instance",
			expected:     "test@my-instance.container",
		},
		{
			name:         "template without @ suffix",
			templateName: "test.container",
			instanceName: "instance1",
			expected:     "test@instance1.container",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateInstanceName(tt.templateName, tt.instanceName)
			require.Equal(t, tt.expected, result)
		})
	}
}
