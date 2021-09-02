module github.com/kedacore/keda-olm-operator

go 1.16

require (
	github.com/go-logr/logr v0.4.0
	github.com/manifestival/controller-runtime-client v0.4.0
	github.com/manifestival/manifestival v0.7.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v3.9.0+incompatible
	k8s.io/api v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v0.22.1
	k8s.io/kube-aggregator v0.22.1
	sigs.k8s.io/controller-runtime v0.10.0
)
