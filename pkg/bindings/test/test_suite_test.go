package test_bindings_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
)

func TestTest(t *testing.T) {
	if testing.Verbose() {
		logrus.SetLevel(logrus.DebugLevel)
	}

	RegisterFailHandler(Fail)
	RunSpecs(t, "Test Suite")
}
