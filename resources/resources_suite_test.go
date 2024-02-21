package resources

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestResourceHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Resource Handler Suite")
}
