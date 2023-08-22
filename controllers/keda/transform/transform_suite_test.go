package transform_test

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	testType string
)

func TestTransform(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Transform Suite")
}

func init() {
	flag.StringVar(&testType, "test.type", "", "type of test: functionality / deployment")
}
