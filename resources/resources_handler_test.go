package resources

import (
	"flag"

	mf "github.com/manifestival/manifestival"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	testType string
)

func init() {
	flag.StringVar(&testType, "test.type", "", "type of test: functionality / deployment")
}

var _ = Describe("Checking KEDA resources manifest", func() {
	When("valid KEDA release is given", func() {
		It("should return valid manifest", func() {
			manifest, err := GetResourcesManifest("")
			Expect(err).To(BeNil())
			Expect(manifest).To(Not(BeNil()))
		})
	})

	When("invalid KEDA release is given", func() {
		It("should return error", func() {
			manifest, err := GetResourcesManifest("keda-not-released")
			Expect(err).To(Not(BeNil()))
			Expect(manifest).To(Equal(mf.Manifest{}))
		})
	})
})

var _ = Describe("Checking KEDA release dir parsing", func() {
	When("empty string is given as KEDA release", func() {
		It("should result in valid release", func() {
			dir, err := parseDirName("")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal(""))
		})
	})

	When("semver with 'v' prefix is given as KEDA release", func() {
		It("should result in valid release and parsed dir does not contain the prefix 'v'", func() {
			dir, err := parseDirName("v2.0.0")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal("2.0.0"))
		})
	})

	When("semver without 'v' prefix is given as KEDA release", func() {
		It("should result in valid release and parsed dir does not contain the prefix 'v'", func() {
			dir, err := parseDirName("2.0.0")
			Expect(err).To(BeNil())
			Expect(dir).To(Equal("2.0.0"))
		})
	})

	When("invalid string is given as KEDA release", func() {
		It("should result in error", func() {
			dir, err := parseDirName("keda-v2.0.0")
			Expect(err).To(Not(BeNil()))
			Expect(dir).To(Equal(""))
		})
	})
})
