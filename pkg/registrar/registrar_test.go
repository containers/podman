package registrar_test

import (
	"testing"

	"github.com/containers/libpod/pkg/registrar"
	. "github.com/containers/libpod/test/framework"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// TestRegistrar runs the created specs
func TestRegistrar(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Registrar")
}

// nolint: gochecknoglobals
var t *TestFramework

var _ = BeforeSuite(func() {
	t = NewTestFramework(NilFunc, NilFunc)
	t.Setup()
})

var _ = AfterSuite(func() {
	t.Teardown()
})

// The actual test suite
var _ = t.Describe("Registrar", func() {
	// Constant test data needed by some tests
	const (
		testKey    = "testKey"
		testName   = "testName"
		anotherKey = "anotherKey"
	)

	// The system under test
	var sut *registrar.Registrar

	// Prepare the system under test and register a test name and key before
	// each test
	BeforeEach(func() {
		sut = registrar.NewRegistrar()
		Expect(sut.Reserve(testName, testKey)).To(BeNil())
	})

	t.Describe("Reserve", func() {
		It("should succeed to reserve a new registrar", func() {
			// Given
			// When
			err := sut.Reserve("name", "key")

			// Then
			Expect(err).To(BeNil())
		})

		It("should succeed to reserve a registrar twice", func() {
			// Given
			// When
			err := sut.Reserve(testName, testKey)

			// Then
			Expect(err).To(BeNil())
		})

		It("should fail to reserve an already reserved registrar", func() {
			// Given
			// When
			err := sut.Reserve(testName, anotherKey)

			// Then
			Expect(err).NotTo(BeNil())
			Expect(err).To(Equal(registrar.ErrNameReserved))
		})
	})

	t.Describe("Release", func() {
		It("should succeed to release a registered registrar multiple times", func() {
			// Given
			// When
			// Then
			sut.Release(testName)
			sut.Release(testName)
		})

		It("should succeed to release a unknown registrar multiple times", func() {
			// Given
			// When
			// Then
			sut.Release(anotherKey)
			sut.Release(anotherKey)
		})

		It("should succeed to release and re-register a registrar", func() {
			// Given
			// When
			sut.Release(testName)
			err := sut.Reserve(testName, testKey)

			// Then
			Expect(err).To(BeNil())
		})
	})

	t.Describe("GetNames", func() {
		It("should succeed to retrieve a single name for a registrar", func() {
			// Given
			// When
			names, err := sut.GetNames(testKey)

			// Then
			Expect(err).To(BeNil())
			Expect(len(names)).To(Equal(1))
			Expect(names[0]).To(Equal(testName))
		})

		It("should succeed to retrieve all names for a registrar", func() {
			// Given
			testNames := []string{"test1", "test2"}
			for _, name := range testNames {
				Expect(sut.Reserve(name, anotherKey)).To(BeNil())
			}

			// When
			names, err := sut.GetNames(anotherKey)

			// Then
			Expect(err).To(BeNil())
			Expect(len(names)).To(Equal(2))
			Expect(names).To(Equal(testNames))
		})
	})

	t.Describe("GetNames", func() {
		It("should succeed to retrieve a single name for a registrar", func() {
			// Given
			// When
			names, err := sut.GetNames(testKey)

			// Then
			Expect(err).To(BeNil())
			Expect(len(names)).To(Equal(1))
			Expect(names[0]).To(Equal(testName))
		})

		It("should succeed to retrieve all names for a registrar", func() {
			// Given
			anotherKey := "anotherKey"
			testNames := []string{"test1", "test2"}
			for _, name := range testNames {
				Expect(sut.Reserve(name, anotherKey)).To(BeNil())
			}

			// When
			names, err := sut.GetNames(anotherKey)

			// Then
			Expect(err).To(BeNil())
			Expect(len(names)).To(Equal(2))
			Expect(names).To(Equal(testNames))
		})
	})

	t.Describe("Delete", func() {
		It("should succeed to delete a registrar", func() {
			// Given
			// When
			sut.Delete(testKey)

			// Then
			names, err := sut.GetNames(testKey)
			Expect(len(names)).To(BeZero())
			Expect(err).To(Equal(registrar.ErrNoSuchKey))
		})
	})

	t.Describe("Get", func() {
		It("should succeed to get a key for a registrar", func() {
			// Given
			// When
			key, err := sut.Get(testName)

			// Then
			Expect(err).To(BeNil())
			Expect(key).To(Equal(testKey))
		})

		It("should fail to get a key for a not existing registrar", func() {
			// Given
			// When
			key, err := sut.Get("notExistingName")

			// Then
			Expect(key).To(BeEmpty())
			Expect(err).To(Equal(registrar.ErrNameNotReserved))
		})
	})

	t.Describe("GetAll", func() {
		It("should succeed to get all names", func() {
			// Given
			// When
			names := sut.GetAll()

			// Then
			Expect(len(names)).To(Equal(1))
			Expect(len(names[testKey])).To(Equal(1))
			Expect(names[testKey][0]).To(Equal(testName))
		})
	})
})
