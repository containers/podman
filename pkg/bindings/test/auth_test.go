package test_bindings

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/containers/common/pkg/auth"
	"github.com/containers/image/v5/types"
	podmanRegistry "github.com/containers/libpod/hack/podman-registry-go"
	"github.com/containers/libpod/pkg/bindings/images"
	"github.com/containers/libpod/pkg/domain/entities"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Podman images", func() {
	var (
		registry *podmanRegistry.Registry
		bt       *bindingTest
		s        *gexec.Session
		err      error
	)

	BeforeEach(func() {
		// Note: we need to start the registry **before** setting up
		// the test. Otherwise, the registry is not reachable for
		// currently unknown reasons.
		registry, err = podmanRegistry.Start()
		Expect(err).To(BeNil())

		bt = newBindingTest()
		bt.RestoreImagesFromCache()
		s = bt.startAPIService()
		time.Sleep(1 * time.Second)
		err := bt.NewConnection()
		Expect(err).To(BeNil())
	})

	AfterEach(func() {
		s.Kill()
		bt.cleanup()
		registry.Stop()
	})

	// Test using credentials.
	It("tag + push + pull (with credentials)", func() {

		imageRep := "localhost:" + registry.Port + "/test"
		imageTag := "latest"
		imageRef := imageRep + ":" + imageTag

		// Tag the alpine image and verify it has worked.
		err = images.Tag(bt.conn, alpine.shortName, imageTag, imageRep)
		Expect(err).To(BeNil())
		_, err = images.GetImage(bt.conn, imageRef, nil)
		Expect(err).To(BeNil())

		// Now push the image.
		pushOpts := entities.ImagePushOptions{
			Username:      registry.User,
			Password:      registry.Password,
			SkipTLSVerify: types.OptionalBoolTrue,
		}
		err = images.Push(bt.conn, imageRef, imageRef, pushOpts)
		Expect(err).To(BeNil())

		// Now pull the image.
		pullOpts := entities.ImagePullOptions{
			Username:      registry.User,
			Password:      registry.Password,
			SkipTLSVerify: types.OptionalBoolTrue,
		}
		_, err = images.Pull(bt.conn, imageRef, pullOpts)
		Expect(err).To(BeNil())
	})

	// Test using authfile.
	It("tag + push + pull + search (with authfile)", func() {

		imageRep := "localhost:" + registry.Port + "/test"
		imageTag := "latest"
		imageRef := imageRep + ":" + imageTag

		// Create a temporary authentication file.
		tmpFile, err := ioutil.TempFile("", "auth.json.")
		Expect(err).To(BeNil())
		_, err = tmpFile.Write([]byte{'{', '}'})
		Expect(err).To(BeNil())
		err = tmpFile.Close()
		Expect(err).To(BeNil())

		authFilePath := tmpFile.Name()

		// Now login to a) test the credentials and to b) store them in
		// the authfile for later use.
		sys := types.SystemContext{
			AuthFilePath:                authFilePath,
			DockerInsecureSkipTLSVerify: types.OptionalBoolTrue,
		}
		loginOptions := auth.LoginOptions{
			Username: registry.User,
			Password: registry.Password,
			AuthFile: authFilePath,
			Stdin:    os.Stdin,
			Stdout:   os.Stdout,
		}
		err = auth.Login(bt.conn, &sys, &loginOptions, []string{imageRep})
		Expect(err).To(BeNil())

		// Tag the alpine image and verify it has worked.
		err = images.Tag(bt.conn, alpine.shortName, imageTag, imageRep)
		Expect(err).To(BeNil())
		_, err = images.GetImage(bt.conn, imageRef, nil)
		Expect(err).To(BeNil())

		// Now push the image.
		pushOpts := entities.ImagePushOptions{
			Authfile:      authFilePath,
			SkipTLSVerify: types.OptionalBoolTrue,
		}
		err = images.Push(bt.conn, imageRef, imageRef, pushOpts)
		Expect(err).To(BeNil())

		// Now pull the image.
		pullOpts := entities.ImagePullOptions{
			Authfile:      authFilePath,
			SkipTLSVerify: types.OptionalBoolTrue,
		}
		_, err = images.Pull(bt.conn, imageRef, pullOpts)
		Expect(err).To(BeNil())

		// Last, but not least, exercise search.
		searchOptions := entities.ImageSearchOptions{
			Authfile:      authFilePath,
			SkipTLSVerify: types.OptionalBoolTrue,
		}
		_, err = images.Search(bt.conn, imageRef, searchOptions)
		Expect(err).To(BeNil())
	})

})
