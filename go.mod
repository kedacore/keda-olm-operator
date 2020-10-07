module github.com/kedacore/keda-olm-operator

go 1.15

require (
	// treba zmenit na 0.1.0
	github.com/go-logr/logr v0.1.0
	// sedi
	github.com/manifestival/controller-runtime-client v0.3.0
	// bolo 0.6.1
	github.com/manifestival/manifestival v0.6.0
	// not in origin
	github.com/onsi/ginkgo v1.12.1
	// not in origin
	github.com/onsi/gomega v1.10.1
	// origin 0.18.6
	// bolo 19.6
	k8s.io/api v0.18.8
	// origin 0.18.6
	// bolo 19.6
	k8s.io/apimachinery v0.18.8
	// origin 0.18.6
	// bolo 19.6
	k8s.io/client-go v0.18.8
	// origin 0.18.6
	// bolo 19.6
	k8s.io/kube-aggregator v0.18.8
	// sedi
	sigs.k8s.io/controller-runtime v0.6.1
	sigs.k8s.io/controller-tools v0.3.0 // indirect
	sigs.k8s.io/kustomize/kustomize/v3 v3.5.4 // indirect
)
