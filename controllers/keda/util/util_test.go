/*
Copyright 2024 The KEDA Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	restclient "k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/kedacore/keda-olm-operator/controllers/keda/util"
)

var _ = Describe("Checking for the OpenShift RuntimeDefault seccomp profile", func() {
	logger := ctrl.Log.WithName("test")
	testData := []struct {
		version           version.Info
		hasRuntimeDefault bool
		context           string
	}{
		{version.Info{Major: "1", Minor: "28", GitCommit: "baz"}, false, "When running on a recent cluster"},
		{version.Info{Major: "1", Minor: "28+", GitCommit: "baz"}, false, "When running on a recent cluster with extra chars in the minor version"},
		{version.Info{Major: "1", Minor: "21", GitCommit: "baz"}, true, "When running on an old cluster"},
		{version.Info{Major: "1", Minor: "", GitCommit: "baz"}, false, "When running on a cluster with no minor version"},
		{version.Info{Major: "1", Minor: "+", GitCommit: "baz"}, false, "When running on a cluster with a garbage minor version"},
	}
	for _, tt := range testData {
		Context(tt.context, func() {
			It("Should be able to determine whether RuntimeDefault exists by getting the cluster version", func() {
				if testType != "unit" {
					Skip("test.type isn't 'unit'")
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					output, err := json.Marshal(tt.version)
					Expect(err).To(BeNil())

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(output)
				}))
				defer server.Close()

				client := discovery.NewDiscoveryClientForConfigOrDie(&restclient.Config{Host: server.URL})
				got := util.RunningOnClusterWithoutSeccompProfileDefault(logger, client)

				Expect(got).To(Equal(tt.hasRuntimeDefault))
			})
		})

	}
})
