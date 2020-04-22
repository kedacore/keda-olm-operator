module github.com/kedacore/keda-olm-operator

go 1.13

require (
	github.com/go-logr/logr v0.1.0
	github.com/go-openapi/spec v0.19.4
	github.com/jcrossley3/manifestival v0.0.0-20191210210340-41c7c3a22365
	github.com/operator-framework/operator-sdk v0.17.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.17.4
	k8s.io/apimachinery v0.17.4
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/kube-aggregator v0.17.3
	k8s.io/kube-openapi v0.0.0-20191107075043-30be4d16710a
	sigs.k8s.io/controller-runtime v0.5.2
)

replace (
	github.com/Azure/go-autorest => github.com/Azure/go-autorest v13.3.2+incompatible // Required by OLM
	k8s.io/client-go => k8s.io/client-go v0.17.4 // Required by prometheus-operator
)
