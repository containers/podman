//go:build linux || freebsd

package integration

import (
	"os"
	"path/filepath"
	"time"

	. "github.com/containers/podman/v5/test/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Podman artifact created timestamp", func() {

	createArtifactFile := func(size int) (string, error) {
		artifactFile := filepath.Join(podmanTest.TempDir, RandomString(12))
		f, err := os.Create(artifactFile)
		if err != nil {
			return "", err
		}
		defer f.Close()

		data := RandomString(size)
		_, err = f.WriteString(data)
		if err != nil {
			return "", err
		}
		return artifactFile, nil
	}

	It("podman artifact inspect shows created date in RFC3339", func() {
		artifactFile1, err := createArtifactFile(1024)
		Expect(err).ToNot(HaveOccurred())
		artifactFile2, err := createArtifactFile(2048)
		Expect(err).ToNot(HaveOccurred())

		artifactName := "localhost/test/artifact-created"

		// Record time before creation (with some buffer for slow systems)
		beforeCreate := time.Now().UTC().Add(-time.Second)

		// Add artifact
		podmanTest.PodmanExitCleanly("artifact", "add", artifactName, artifactFile1)

		// Record time after creation
		afterCreate := time.Now().UTC().Add(time.Second)

		// Inspect artifact
		a := podmanTest.InspectArtifact(artifactName)
		Expect(a.Name).To(Equal(artifactName))

		// Check that created annotation exists and is in valid RFC3339 format
		createdStr, exists := a.Manifest.Annotations["org.opencontainers.image.created"]
		Expect(exists).To(BeTrue(), "Should have org.opencontainers.image.created annotation")

		// Parse the created timestamp as RFC3339Nano
		createdTime, err := time.Parse(time.RFC3339Nano, createdStr)
		Expect(err).ToNot(HaveOccurred(), "Created timestamp should be valid RFC3339Nano format")

		// Verify timestamp is reasonable (within our time window)
		Expect(createdTime).To(BeTemporally(">=", beforeCreate))
		Expect(createdTime).To(BeTemporally("<=", afterCreate))

		// Wait a moment to ensure timestamps would be different
		time.Sleep(100 * time.Millisecond)

		// Append to the artifact
		podmanTest.PodmanExitCleanly("artifact", "add", "--append", artifactName, artifactFile2)

		// Check that created timestamp is unchanged
		a = podmanTest.InspectArtifact(artifactName)
		currentCreated, exists := a.Manifest.Annotations["org.opencontainers.image.created"]
		Expect(exists).To(BeTrue(), "Should have org.opencontainers.image.created annotation")

		Expect(currentCreated).To(Equal(createdStr), "Created timestamp should not change when appending")

		// Verify we have 2 layers
		Expect(a.Manifest.Layers).To(HaveLen(2))
	})
})
