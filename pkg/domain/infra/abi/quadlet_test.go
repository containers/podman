//go:build !remote

package abi

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectQuadletType(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    string
		expectError bool
	}{
		{
			name: "container quadlet",
			content: `[Container]
Image=localhost/imagename`,
			expected:    ".container",
			expectError: false,
		},
		{
			name:        "volume quadlet",
			content:     `[Volume]`,
			expected:    ".volume",
			expectError: false,
		},
		{
			name:        "network quadlet",
			content:     `[Network]`,
			expected:    ".network",
			expectError: false,
		},
		{
			name: "kube quadlet",
			content: `[Kube]
Yaml=test.yaml`,
			expected:    ".kube",
			expectError: false,
		},
		{
			name: "image quadlet",
			content: `[Image]
Image=localhost/imagename`,
			expected:    ".image",
			expectError: false,
		},
		{
			name: "build quadlet",
			content: `[Build]
File=Containerfile`,
			expected:    ".build",
			expectError: false,
		},
		{
			name:        "pod quadlet",
			content:     `[Pod]`,
			expected:    ".pod",
			expectError: false,
		},
		{
			name: "mixed case container",
			content: `[CONTAINER]
Image=localhost/imagename`,
			expected:    ".container",
			expectError: false,
		},
		{
			name: "container with other sections",
			content: `[Unit]
Description=Test container

[Container]
Image=localhost/imagename

[Service]
Restart=always`,
			expected:    ".container",
			expectError: false,
		},
		{
			name: "no quadlet section",
			content: `[Unit]
Description=Test unit

[Service]
ExecStart=/bin/echo hello`,
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty content",
			content:     "",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := detectQuadletType(tt.content)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestIsMultiQuadletFile(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name: "single quadlet",
			content: `[Container]
Image=localhost/imagename`,
			expected: false,
		},
		{
			name: "multi quadlet with separator",
			content: `[Container]
Image=localhost/imagename
---
[Volume]`,
			expected: true,
		},
		{
			name: "separator in comment",
			content: `[Container]
Image=localhost/imagename
# This is not a separator: ---`,
			expected: false,
		},
		{
			name: "separator on own line",
			content: `[Container]
Image=localhost/imagename
---
[Volume]`,
			expected: true,
		},
		{
			name: "multiple separators",
			content: `[Container]
Image=localhost/imagename
---
[Volume]
---
[Network]`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the content
			tmpDir := t.TempDir()
			tmpFile := tmpDir + "/quadlet-test.txt"
			err := os.WriteFile(tmpFile, []byte(tt.content), 0644)
			require.NoError(t, err)

			result, err := isMultiQuadletFile(tmpFile)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseMultiQuadletFile(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedCount int
		expectedNames []string
		expectedExts  []string
		expectError   bool
	}{
		{
			name: "single container quadlet",
			content: `[Container]
Image=localhost/imagename`,
			expectedCount: 1,
			expectedNames: []string{"test"},
			expectedExts:  []string{".container"},
			expectError:   false,
		},
		{
			name: "multiple quadlets with FileName comments",
			content: `# FileName=web-server
[Container]
Image=localhost/imagename
---
# FileName=app-storage
[Volume]
---
# FileName=app-network
[Network]`,
			expectedCount: 3,
			expectedNames: []string{"web-server", "app-storage", "app-network"},
			expectedExts:  []string{".container", ".volume", ".network"},
			expectError:   false,
		},
		{
			name: "quadlets with empty sections",
			content: `# FileName=web-container
[Container]
Image=localhost/imagename
---

---
# FileName=data-volume
[Volume]`,
			expectedCount: 2,
			expectedNames: []string{"web-container", "data-volume"},
			expectedExts:  []string{".container", ".volume"},
			expectError:   false,
		},
		{
			name: "multiple quadlets missing FileName",
			content: `[Container]
Image=localhost/imagename
---
[Volume]`,
			expectedCount: 0,
			expectedNames: nil,
			expectedExts:  nil,
			expectError:   true,
		},
		{
			name: "invalid quadlet section",
			content: `# FileName=test-container
[Container]
Image=localhost/imagename
---
# FileName=invalid-section
[InvalidSection]
SomeKey=value`,
			expectedCount: 0,
			expectedNames: nil,
			expectedExts:  nil,
			expectError:   true,
		},
		{
			name:          "empty file",
			content:       "",
			expectedCount: 0,
			expectedNames: nil,
			expectedExts:  nil,
			expectError:   true,
		},
		{
			name: "only separators",
			content: `---
---`,
			expectedCount: 0,
			expectedNames: nil,
			expectedExts:  nil,
			expectError:   true,
		},
		{
			name: "FileName with invalid characters",
			content: `# FileName=web/server
[Container]
Image=localhost/imagename
---
# FileName=app-storage
[Volume]`,
			expectedCount: 0,
			expectedNames: nil,
			expectedExts:  nil,
			expectError:   true,
		},
		{
			name: "FileName with extension",
			content: `# FileName=web-server.container
[Container]
Image=localhost/imagename
---
# FileName=app-storage
[Volume]`,
			expectedCount: 0,
			expectedNames: nil,
			expectedExts:  nil,
			expectError:   true,
		},
		{
			name: "empty FileName",
			content: `# FileName=
[Container]
Image=localhost/imagename
---
# FileName=app-storage
[Volume]`,
			expectedCount: 0,
			expectedNames: nil,
			expectedExts:  nil,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file with the content
			tmpDir := t.TempDir()
			tmpFile := tmpDir + "/test.txt"
			err := os.WriteFile(tmpFile, []byte(tt.content), 0644)
			require.NoError(t, err)

			result, err := parseMultiQuadletFile(tmpFile)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)

				for i, quadlet := range result {
					assert.Equal(t, tt.expectedNames[i], quadlet.name)
					assert.Equal(t, tt.expectedExts[i], quadlet.extension)
					assert.NotEmpty(t, quadlet.content)
				}
			}
		})
	}
}

func TestExtractFileNameFromSection(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		expected    string
		expectError bool
	}{
		{
			name: "valid FileName comment",
			content: `# FileName=web-server
[Container]
Image=nginx:latest`,
			expected:    "web-server",
			expectError: false,
		},
		{
			name: "FileName with spaces around",
			content: `#   FileName=my-app
[Container]
Image=nginx:latest`,
			expected:    "my-app",
			expectError: false,
		},
		{
			name: "FileName in middle of section",
			content: `[Container]
# FileName=middle-name
Image=nginx:latest`,
			expected:    "middle-name",
			expectError: false,
		},
		{
			name: "missing FileName comment",
			content: `[Container]
Image=nginx:latest`,
			expected:    "",
			expectError: true,
		},
		{
			name: "empty FileName",
			content: `# FileName=
[Container]
Image=nginx:latest`,
			expected:    "",
			expectError: true,
		},
		{
			name: "FileName with path separator",
			content: `# FileName=web/server
[Container]
Image=nginx:latest`,
			expected:    "",
			expectError: true,
		},
		{
			name: "FileName with extension",
			content: `# FileName=web-server.container
[Container]
Image=nginx:latest`,
			expected:    "",
			expectError: true,
		},
		{
			name: "multiple FileName comments (first one wins)",
			content: `# FileName=first-name
[Container]
# FileName=second-name
Image=nginx:latest`,
			expected:    "first-name",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractFileNameFromSection(tt.content)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseMultiQuadletFileRealExample(t *testing.T) {
	// Test with a realistic multi-quadlet file
	content := `# FileName=web-server
# Web application stack
[Container]
Image=nginx:latest
ContainerName=web-server
PublishPort=8080:80
Volume=web-content:/usr/share/nginx/html:Z

---

# FileName=app-storage
# Database volume
[Volume]
Label=app=webapp
Label=component=database

---

# FileName=app-network
# Application network
[Network]
Subnet=10.0.0.0/24
Gateway=10.0.0.1
Label=app=webapp`

	// Create a temporary file
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/webapp.quadlets"
	err := os.WriteFile(tmpFile, []byte(content), 0644)
	require.NoError(t, err)

	// Parse the file
	quadlets, err := parseMultiQuadletFile(tmpFile)
	require.NoError(t, err)
	assert.Len(t, quadlets, 3)

	// Check first quadlet (container)
	assert.Equal(t, "web-server", quadlets[0].name)
	assert.Equal(t, ".container", quadlets[0].extension)
	assert.Contains(t, quadlets[0].content, "[Container]")
	assert.Contains(t, quadlets[0].content, "Image=nginx:latest")

	// Check second quadlet (volume)
	assert.Equal(t, "app-storage", quadlets[1].name)
	assert.Equal(t, ".volume", quadlets[1].extension)
	assert.Contains(t, quadlets[1].content, "[Volume]")
	assert.Contains(t, quadlets[1].content, "Label=component=database")

	// Check third quadlet (network)
	assert.Equal(t, "app-network", quadlets[2].name)
	assert.Equal(t, ".network", quadlets[2].extension)
	assert.Contains(t, quadlets[2].content, "[Network]")
	assert.Contains(t, quadlets[2].content, "Subnet=10.0.0.0/24")
}
