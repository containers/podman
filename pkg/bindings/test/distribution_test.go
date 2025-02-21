package bindings_test

import (
	"time"

	"github.com/containers/podman/v5/pkg/bindings/images"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman distribution", func() {
	var (
		bt *bindingTest
		s  *Session
	)

	BeforeEach(func() {
		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
	})

	It("inspect distribution data", func() {
		distributionInfo, err := images.GetDistribution(bt.conn, alpine.name, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(distributionInfo).ToNot(BeNil())

		Expect(distributionInfo.Descriptor.MediaType).ToNot(BeEmpty())
		Expect(distributionInfo.Descriptor.Digest).ToNot(BeEmpty())
		Expect(distributionInfo.Descriptor.Size).To(BeNumerically(">", 0))

		Expect(distributionInfo.Platforms).ToNot(BeEmpty())
		for _, platform := range distributionInfo.Platforms {
			Expect(platform.OS).ToNot(BeEmpty())
			Expect(platform.Architecture).ToNot(BeEmpty())
		}
	})

	It("inspect distribution data with invalid image", func() {
		_, err := images.GetDistribution(bt.conn, "invalid_image_name:latest", nil)
		Expect(err).To(HaveOccurred())
	})
})
