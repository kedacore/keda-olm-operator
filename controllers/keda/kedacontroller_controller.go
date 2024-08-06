/*
Copyright 2020 The KEDA Authors

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

package keda

import (
	"context"
	"crypto/x509"
	goerrors "errors"
	"fmt"
	"os"
	"path"
	"reflect"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	"github.com/open-policy-agent/cert-controller/pkg/rotator"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/client-go/discovery"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	kedav1alpha1 "github.com/kedacore/keda-olm-operator/apis/keda/v1alpha1"
	"github.com/kedacore/keda-olm-operator/controllers/keda/transform"
	"github.com/kedacore/keda-olm-operator/controllers/keda/util"
	"github.com/kedacore/keda-olm-operator/resources"
	"github.com/kedacore/keda-olm-operator/version"
)

const (

	// Allowed Name of KedaController resource
	kedaControllerResourceName = "keda"

	grpcClientCertsSecretName     = "kedaorg-certs"
	caBundleConfigMapName         = "keda-ocp-cabundle"
	injectCABundleAnnotation      = "service.beta.openshift.io/inject-cabundle"
	injectCABundleAnnotationValue = "true"
	servingCertsAnnotation        = "service.beta.openshift.io/serving-cert-secret-name"
	roleBindingName               = "keda-auth-reader"
	roleBindingNamespace          = "kube-system"

	auditlogPolicyConfigMap = "keda-metrics-server-audit-policy"
	auditlogPolicyMountPath = "/var/audit-policy"
	auditPolicyFile         = "policy.yaml"
)

// KedaControllerReconciler reconciles a KedaController object
type KedaControllerReconciler struct {
	client.Client
	Log                 logr.Logger
	Scheme              *runtime.Scheme
	CertDir             string
	LeaderElection      bool
	rotatorStarted      bool
	mgr                 ctrl.Manager
	resourcesGeneral    mf.Manifest
	resourcesController mf.Manifest
	resourcesMetrics    mf.Manifest
	resourcesWebhooks   mf.Manifest
	resourcesMonitoring mf.Manifest
	discoveryClient     *discovery.DiscoveryClient
	resourceNamespace   string
}

func (r *KedaControllerReconciler) SetupWithManager(mgr ctrl.Manager, kedaControllerResourceNamespace string, logger logr.Logger) error {
	r.resourceNamespace = kedaControllerResourceNamespace
	r.mgr = mgr
	resourcesManifest, err := resources.GetResourcesManifest()
	if err != nil {
		return err
	}
	manifestGeneral, manifestController, manifestMetrics, manifestWebhooks, manifestMonitoring, err := parseManifestsFromFile(resourcesManifest, r.Client)
	if err != nil {
		return err
	}

	r.resourcesGeneral = manifestGeneral
	r.resourcesController = manifestController
	r.resourcesMetrics = manifestMetrics
	r.resourcesWebhooks = manifestWebhooks
	r.resourcesMonitoring = manifestMonitoring
	if restConfig, err := ctrl.GetConfig(); err != nil {
		logger.Info("Unable to get REST Config for cluster version discovery. Ignore this message in test environments", "err", err)
	} else {
		if r.discoveryClient, err = discovery.NewDiscoveryClientForConfig(restConfig); err != nil {
			logger.Info("Unable to get discovery client for cluster version discovery. Ignore this message in test environments", "err", err)
		}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kedav1alpha1.KedaController{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=keda.sh,resources=kedacontrollers;kedacontrollers/finalizers;kedacontrollers/status,verbs="*"
// +kubebuilder:rbac:groups="",resources=namespaces;serviceaccounts;pods;services;services/finalizers;endpoints;persistentvolumeclaims;events;configmaps;secrets,verbs="*"
// +kubebuilder:rbac:groups=apps,resources=deployments;daemonsets;replicasets;statefulsets,verbs="*"
// +kubebuilder:rbac:groups=apps,resourceNames=keda-olm-operator,resources=deployments/finalizers,verbs="*"
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings;clusterroles;rolebindings;roles,verbs="*"
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=validatingwebhookconfigurations,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=apiregistration.k8s.io,resources=apiservices,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=servicemonitors;podmonitors,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=list
// +kubebuilder:rbac:groups="coordination.k8s.io",resources=leases,verbs="*"

// Reconcile reads that state of the cluster for a KedaController object and makes changes based on the state read
// and what is in the KedaController.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *KedaControllerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("KedaController", req.NamespacedName)

	logger.Info("Reconciling KedaController")

	// Fetch the KedaController instance
	instance := &kedav1alpha1.KedaController{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	if !isInteresting(req, r.resourceNamespace) {
		msg := fmt.Sprintf("The KedaController resource needs to be created in namespace %s with name %s, otherwise it will be ignored", r.resourceNamespace, kedaControllerResourceName)
		logger.Info(msg)
		status := instance.Status.DeepCopy()
		status.MarkIgnored(msg)
		return ctrl.Result{}, util.UpdateKedaControllerStatus(ctx, r.Client, instance, status)
	}

	if instance.GetDeletionTimestamp() != nil {
		if contains(instance.GetFinalizers(), kedaControllerFinalizer) {
			// Run finalization logic for kedaControllerFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeKedaController(logger); err != nil {
				return ctrl.Result{}, err
			}
			// Remove kedaControllerFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			patch := client.MergeFrom(instance.DeepCopy())
			instance.SetFinalizers(remove(instance.GetFinalizers(), kedaControllerFinalizer))
			err := r.Client.Patch(ctx, instance, patch)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), kedaControllerFinalizer) {
		if err := r.addFinalizer(ctx, logger, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	status := instance.Status.DeepCopy()

	if err := r.installSA(logger, instance); err != nil {
		status.MarkInstallFailed("Not able to create ServiceAccount")
		if statusErr := util.UpdateKedaControllerStatus(ctx, r.Client, instance, status); statusErr != nil {
			err = fmt.Errorf("got error: %s and then another: %s", err, statusErr)
		}
		return ctrl.Result{}, err
	}
	if err := r.installController(ctx, logger, instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA Controller")
		if statusErr := util.UpdateKedaControllerStatus(ctx, r.Client, instance, status); statusErr != nil {
			err = fmt.Errorf("got error: %s and then another: %s", err, statusErr)
		}
		return ctrl.Result{}, err
	}
	if err := r.installMetricsServer(ctx, logger, instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA Metrics Server")
		if statusErr := util.UpdateKedaControllerStatus(ctx, r.Client, instance, status); statusErr != nil {
			err = fmt.Errorf("got error: %s and then another: %s", err, statusErr)
		}
		return ctrl.Result{}, err
	}

	if err := r.installAdmissionWebhooks(ctx, logger, instance); err != nil {
		status.MarkInstallFailed("Not able to install KEDA Admission Webhooks")
		if statusErr := util.UpdateKedaControllerStatus(ctx, r.Client, instance, status); statusErr != nil {
			err = fmt.Errorf("got error: %s and then another: %s", err, statusErr)
		}
		return ctrl.Result{}, err
	}

	if err := r.installMonitoring(ctx, logger, instance); err != nil {
		status.MarkInstallFailed("Not able to install monitoring resources")
		if statusErr := util.UpdateKedaControllerStatus(ctx, r.Client, instance, status); statusErr != nil {
			err = fmt.Errorf("got error: %s and then another: %s", err, statusErr)
		}
		return ctrl.Result{}, err
	}

	status.Version = version.Version
	status.MarkInstallSucceeded(fmt.Sprintf("KEDA v%s is installed in namespace '%s'", version.Version, r.resourceNamespace))
	if err := util.UpdateKedaControllerStatus(ctx, r.Client, instance, status); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func parseManifestsFromFile(manifest mf.Manifest, c client.Client) (manifestGeneral, manifestController,
	manifestMetrics, manifestWebhook, manifestMonitoring mf.Manifest, err error) {
	var generalResources, controllerResources, metricsResources, webhookResources, monitoringResources []unstructured.Unstructured

	for _, r := range manifest.Resources() {
		switch kind := r.GetKind(); kind {
		case "APIService", "ValidatingWebhookConfiguration":
			if name := r.GetName(); name == "keda-admission-webhooks" || name == "keda-admission" {
				webhookResources = append(webhookResources, r)
			} else {
				metricsResources = append(metricsResources, r)
			}
		case "ClusterRole", "ClusterRoleBinding", "Deployment", "Role", "RoleBinding", "Service":
			if name := r.GetName(); name == "keda-operator" {
				controllerResources = append(controllerResources, r)
			} else if name := r.GetName(); name == "keda-admission-webhooks" || name == "keda-admission" {
				webhookResources = append(webhookResources, r)
			} else {
				metricsResources = append(metricsResources, r)
			}
		case "Secret":
			controllerResources = append(controllerResources, r)
		case "ServiceAccount":
			generalResources = append(generalResources, r)
		case "PodMonitor", "ServiceMonitor":
			monitoringResources = append(monitoringResources, r)
		case "Namespace":
			// ignore. we don't need to create or label the namespace since it's a prereq for OLM install anyway
		}
	}

	manifestClient := mfc.NewClient(c)
	manifestGeneral, err = mf.ManifestFrom(mf.Slice(generalResources), mf.UseLastAppliedConfigAnnotation(resources.LastConfigID))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestGeneral.Client = manifestClient

	manifestController, err = mf.ManifestFrom(mf.Slice(controllerResources), mf.UseLastAppliedConfigAnnotation(resources.LastConfigID))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestController.Client = manifestClient

	manifestMetrics, err = mf.ManifestFrom(mf.Slice(sortMetricsResources(&metricsResources)), mf.UseLastAppliedConfigAnnotation(resources.LastConfigID))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestMetrics.Client = manifestClient

	manifestWebhook, err = mf.ManifestFrom(mf.Slice(webhookResources), mf.UseLastAppliedConfigAnnotation(resources.LastConfigID))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestWebhook.Client = manifestClient

	manifestMonitoring, err = mf.ManifestFrom(mf.Slice(monitoringResources), mf.UseLastAppliedConfigAnnotation(resources.LastConfigID))
	if err != nil {
		return mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, mf.Manifest{}, err
	}
	manifestMonitoring.Client = manifestClient

	return
}

func sortMetricsResources(resources *[]unstructured.Unstructured) []unstructured.Unstructured {
	sortedResources := make([]unstructured.Unstructured, 7)

	for _, r := range *resources {
		switch kind := r.GetKind(); kind {
		case "ClusterRole":
			sortedResources[0] = r
		case "RoleBinding":
			sortedResources[1] = r
		case "ClusterRoleBinding":
			if name := r.GetName(); name == "keda-hpa-controller-external-metrics" {
				sortedResources[2] = r
			} else {
				sortedResources[3] = r
			}
		case "Service":
			sortedResources[4] = r
		case "Deployment":
			sortedResources[5] = r
		case "APIService":
			sortedResources[6] = r
		}
	}

	return sortedResources
}

func (r *KedaControllerReconciler) installSA(logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling KEDA ServiceAccount")
	transforms := []mf.Transformer{
		transform.InjectOwner(instance),
		transform.ReplaceAllNamespaces(instance.Namespace),
	}

	if len(instance.Spec.ServiceAccount.Annotations) > 0 {
		transforms = append(transforms, transform.AddServiceAccountAnnotations(instance.Spec.ServiceAccount.Annotations, r.Scheme))
	}

	if len(instance.Spec.ServiceAccount.Labels) > 0 {
		transforms = append(transforms, transform.AddServiceAccountLabels(instance.Spec.ServiceAccount.Labels, r.Scheme))
	}

	manifest, err := r.resourcesGeneral.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform ServiceAccount manifest")
		return err
	}
	r.resourcesGeneral = manifest

	if err := r.resourcesGeneral.Apply(); err != nil {
		logger.Error(err, "Unable to install ServiceAccount")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) installController(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling KEDA Controller deployment")
	transforms := []mf.Transformer{
		transform.InjectOwner(instance),
		transform.ReplaceAllNamespaces(instance.Namespace),
		transform.ReplaceWatchNamespace(instance.Spec.WatchNamespace, "keda-operator", r.Scheme, logger),
	}

	runningOnOpenshift := util.RunningOnOpenshift(ctx, logger, r.Client)

	caConfigMaps := instance.Spec.Operator.CAConfigMaps
	if runningOnOpenshift {
		found := false
		for _, cmName := range caConfigMaps {
			if cmName == caBundleConfigMapName {
				found = true
				break
			}
		}
		if !found {
			// prepend it
			caConfigMaps = append([]string{caBundleConfigMapName}, caConfigMaps...)
		}
	}

	transforms = append(transforms, transform.EnsureCACertsForOperatorDeployment(caConfigMaps, r.Scheme, logger)...)

	if runningOnOpenshift {
		// certificates rotation works only on Openshift due to openshift/service-ca-operator
		serviceName := "keda-operator"
		certsSecretName := serviceName + "-certs"
		transforms = append(transforms,
			transform.EnsureCertInjectionForService(serviceName, servingCertsAnnotation, certsSecretName),
			transform.KedaOperatorEnsureCertificatesVolume(certsSecretName, grpcClientCertsSecretName, r.Scheme),
			transform.SetOperatorCertRotation(false, r.Scheme, logger), // don't use KEDA operator's built-in cert rotation when on OpenShift
		)
		// on OpenShift 4.10 (kube 1.23) and earlier, the RuntimeDefault SeccompProfile won't validate against any SCC
		if util.RunningOnClusterWithoutSeccompProfileDefault(logger, r.discoveryClient) {
			transforms = append(transforms, transform.RemoveSeccompProfileFromKedaOperator(r.Scheme, logger))
		}
	} else {
		transforms = append(transforms,
			transform.SetOperatorCertRotation(true, r.Scheme, logger), // use KEDA operator's built-in cert rotation when not on OpenShift
		)
	}

	// Use alternate image spec if env var set
	if controllerImage := os.Getenv("KEDA_OPERATOR_IMAGE"); len(controllerImage) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorImage(controllerImage, r.Scheme))
	}

	if len(instance.Spec.Operator.LogLevel) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorLogLevel(instance.Spec.Operator.LogLevel, r.Scheme, logger))
	}
	if len(instance.Spec.Operator.LogEncoder) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorLogEncoder(instance.Spec.Operator.LogEncoder, r.Scheme, logger))
	}
	if len(instance.Spec.Operator.LogTimeEncoding) > 0 {
		transforms = append(transforms, transform.ReplaceKedaOperatorLogTimeEncoding(instance.Spec.Operator.LogTimeEncoding, r.Scheme, logger))
	}

	if len(instance.Spec.Operator.DeploymentAnnotations) > 0 {
		transforms = append(transforms, transform.AddDeploymentAnnotations(instance.Spec.Operator.DeploymentAnnotations, r.Scheme))
	}

	if len(instance.Spec.Operator.DeploymentLabels) > 0 {
		transforms = append(transforms, transform.AddDeploymentLabels(instance.Spec.Operator.DeploymentLabels, r.Scheme))
	}

	if len(instance.Spec.Operator.PodAnnotations) > 0 {
		transforms = append(transforms, transform.AddPodAnnotations(instance.Spec.Operator.PodAnnotations, r.Scheme))
	}

	if len(instance.Spec.Operator.PodLabels) > 0 {
		transforms = append(transforms, transform.AddPodLabels(instance.Spec.Operator.PodLabels, r.Scheme))
	}

	if len(instance.Spec.Operator.NodeSelector) > 0 {
		transforms = append(transforms, transform.ReplaceNodeSelector(instance.Spec.Operator.NodeSelector, r.Scheme))
	}

	if len(instance.Spec.Operator.Tolerations) > 0 {
		transforms = append(transforms, transform.ReplaceTolerations(instance.Spec.Operator.Tolerations, r.Scheme))
	}

	if instance.Spec.Operator.Affinity != nil {
		transforms = append(transforms, transform.ReplaceAffinity(instance.Spec.Operator.Affinity, r.Scheme))
	}

	if len(instance.Spec.Operator.PriorityClassName) > 0 {
		transforms = append(transforms, transform.ReplacePriorityClassName(instance.Spec.Operator.PriorityClassName, r.Scheme))
	}

	if instance.Spec.Operator.Resources.Limits != nil || instance.Spec.Operator.Resources.Requests != nil {
		transforms = append(transforms, transform.ReplaceKedaOperatorResources(instance.Spec.Operator.Resources, r.Scheme))
	}

	// add arbitrary args defined by user
	for i := range instance.Spec.Operator.Args {
		i := i
		transforms = append(transforms, transform.ReplaceArbitraryArg(instance.Spec.Operator.Args[i], "operator", r.Scheme, logger))
	}

	manifest, err := r.resourcesController.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform KEDA Controller manifest")
		return err
	}
	r.resourcesController = manifest

	if err := r.resourcesController.Apply(); err != nil {
		logger.Error(err, "Unable to install KEDA Controller")
		return err
	}

	if runningOnOpenshift && !r.rotatorStarted {
		err = rotator.AddRotator(r.mgr, &rotator.CertRotator{
			SecretKey: types.NamespacedName{
				Namespace: r.resourceNamespace,
				Name:      grpcClientCertsSecretName,
			},
			// The 3 values for SecretKey.Name above and CAName, CAOrganization below match the names the KEDA operator uses to generate its certificate
			CAName:                "KEDA",
			CAOrganization:        "KEDAORG",
			CertDir:               r.CertDir,
			IsReady:               make(chan struct{}),
			RequireLeaderElection: r.LeaderElection,
			ExtKeyUsages: &[]x509.ExtKeyUsage{
				x509.ExtKeyUsageClientAuth,
			},
		})
		if err != nil {
			return err
		}
		r.rotatorStarted = true
	}

	return nil
}

// installMonitoring install the controller resources for the monitoring stack
func (r *KedaControllerReconciler) installMonitoring(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling monitoring resources")

	// this works only if required CRDs are present
	if !util.HasServiceMonitorCRD(ctx, logger, r.Client) {
		logger.V(4).Info("ServiceMonitor CRD not found, skipping monitoring resources")
		return nil
	}
	if !util.HasPodMonitorCRD(ctx, logger, r.Client) {
		logger.V(4).Info("PodMonitor CRD not found, skipping monitoring resources")
		return nil
	}

	transforms := []mf.Transformer{
		transform.InjectOwner(instance),
		transform.ReplaceAllNamespaces(instance.Namespace),
	}

	manifest, err := r.resourcesMonitoring.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform monitoring resource manifests")
		return err
	}
	r.resourcesMonitoring = manifest

	if err := r.resourcesMonitoring.Apply(); err != nil {
		logger.Error(err, "Unable to install monitoring resources")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) installMetricsServer(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling KEDA Metrics Server Deployment")

	transforms := []mf.Transformer{
		transform.InjectOwner(instance),
		transform.ReplaceAllNamespaces(instance.Namespace),
	}

	// Use alternate image spec if env var set
	if controllerImage := os.Getenv("KEDA_METRICS_SERVER_IMAGE"); len(controllerImage) > 0 {
		transforms = append(transforms, transform.ReplaceMetricsServerImage(controllerImage, r.Scheme))
	}

	// on OpenShift 4.10 (kube 1.23) and earlier, the RuntimeDefault SeccompProfile won't validate against any SCC
	if util.RunningOnOpenshift(ctx, logger, r.Client) && util.RunningOnClusterWithoutSeccompProfileDefault(logger, r.discoveryClient) {
		transforms = append(transforms, transform.RemoveSeccompProfileFromMetricsServer(r.Scheme, logger))
	}

	// certificates rotation works only on Openshift due to openshift/service-ca-operator
	if util.RunningOnOpenshift(ctx, logger, r.Client) {
		if err := r.ensureOpenshiftCABundleConfigMap(ctx, logger, instance); err != nil {
			logger.Error(err, "Unable to check OpenShift CA Bundle ConfigMap is present")
			return err
		}

		argsPrefixes := []transform.Prefix{transform.ClientCAFile, transform.TLSCertFile, transform.TLSPrivateKeyFile}
		newArgs := []string{"/certs/ca.crt", "/certs/ocp-tls.crt", "/certs/ocp-tls.key"}

		serviceName := "keda-metrics-apiserver"
		certsSecretName := serviceName + "-certs"

		transforms = append(transforms,
			transform.EnsureCABundleInjectionForAPIService(injectCABundleAnnotation, injectCABundleAnnotationValue, r.Scheme),
			transform.EnsureCertInjectionForService(serviceName, servingCertsAnnotation, certsSecretName),
			transform.MetricsServerEnsureCertificatesVolume(caBundleConfigMapName, certsSecretName, r.Scheme),
		)
		transforms = append(transforms, transform.EnsurePathsToCertsInDeployment(newArgs, argsPrefixes, r.Scheme, logger)...)
	} else {
		logger.Info("Not running on OpenShift -> using only KEDA Operator generated self-signed cert for KEDA Metrics Server")
	}

	// Audit logging validation - configMap exists, logOutVolumeClaim validation
	// policy is a wrapper AuditPolicy for auditv1.Policy for easier user exposure
	policy := instance.Spec.MetricsServer.AuditConfig.Policy
	logOutVolumeClaim := instance.Spec.MetricsServer.AuditConfig.LogOutputVolumeClaim

	// if policy is not empty, audit logging is ON
	if !reflect.DeepEqual(policy, kedav1alpha1.AuditPolicy{}) {
		// --- Policy configMap setup ---
		logger.Info("Ensure Audit log Policy ConfigMap for Metrics Server exists")
		err := r.ensureMetricsServerAuditLogPolicyConfigMap(ctx, logger, instance)
		if err != nil {
			logger.Error(err, "unable to check Metrics Server Auditlog Policy ConfigMap is present")
			return err
		}
		transforms = append(transforms, transform.EnsureAuditPolicyConfigMapMountsVolume(auditlogPolicyConfigMap, r.Scheme))
		// add mounted policy file path to MetricsServer arguments
		auditFilePath := path.Join(auditlogPolicyMountPath, auditPolicyFile)
		transforms = append(transforms, transform.ReplaceAuditConfig(auditFilePath, "policyfile", r.Scheme, logger))

		// --- Log output setup ---
		// validation checks around logOutVolumeClaim and lifetime arguments
		if err := validateAuditLogVolumeWithArgs(logOutVolumeClaim, instance.Spec.MetricsServer.AuditConfig.AuditLifetime); err != nil {
			logger.Error(err, "unable to validate args for Audit logging")
			return err
		}

		var logVolumePath string
		if logOutVolumeClaim == "" {
			// if volume is empty -> log to STDOUT
			logVolumePath = "-"
		} else {
			logger.Info("Check if audit log output volume exists")
			err = r.checkAuditLogVolumeExists(logOutVolumeClaim, ctx, instance)
			if err != nil {
				logger.Error(err, "unable to validate log output persistent volume")
				return err
			}
			logVolumePath = "/var/audit-policy/log-" + time.Now().Format("2006.01.02-15:04")
			transforms = append(transforms, transform.EnsureAuditLogMount(logOutVolumeClaim, logVolumePath, r.Scheme))
		}

		// add audit log output volume path to arguments of keda-adapter
		logFullPath := path.Join(logVolumePath, logOutVolumeClaim)
		transforms = append(transforms, transform.ReplaceAuditConfig(logFullPath, "logpath", r.Scheme, logger))
	}

	if len(instance.Spec.MetricsServer.LogLevel) > 0 {
		transforms = append(transforms, transform.ReplaceMetricsServerLogLevel(instance.Spec.MetricsServer.LogLevel, r.Scheme, logger))
	}

	if len(instance.Spec.MetricsServer.DeploymentAnnotations) > 0 {
		transforms = append(transforms, transform.AddDeploymentAnnotations(instance.Spec.MetricsServer.DeploymentAnnotations, r.Scheme))
	}

	if len(instance.Spec.MetricsServer.DeploymentLabels) > 0 {
		transforms = append(transforms, transform.AddDeploymentLabels(instance.Spec.MetricsServer.DeploymentLabels, r.Scheme))
	}

	if len(instance.Spec.MetricsServer.PodAnnotations) > 0 {
		transforms = append(transforms, transform.AddPodAnnotations(instance.Spec.MetricsServer.PodAnnotations, r.Scheme))
	}

	if len(instance.Spec.MetricsServer.PodLabels) > 0 {
		transforms = append(transforms, transform.AddPodLabels(instance.Spec.MetricsServer.PodLabels, r.Scheme))
	}

	if len(instance.Spec.MetricsServer.NodeSelector) > 0 {
		transforms = append(transforms, transform.ReplaceNodeSelector(instance.Spec.MetricsServer.NodeSelector, r.Scheme))
	}

	if len(instance.Spec.MetricsServer.Tolerations) > 0 {
		transforms = append(transforms, transform.ReplaceTolerations(instance.Spec.MetricsServer.Tolerations, r.Scheme))
	}

	if instance.Spec.MetricsServer.Affinity != nil {
		transforms = append(transforms, transform.ReplaceAffinity(instance.Spec.MetricsServer.Affinity, r.Scheme))
	}

	if len(instance.Spec.MetricsServer.PriorityClassName) > 0 {
		transforms = append(transforms, transform.ReplacePriorityClassName(instance.Spec.MetricsServer.PriorityClassName, r.Scheme))
	}

	if instance.Spec.MetricsServer.Resources.Limits != nil || instance.Spec.MetricsServer.Resources.Requests != nil {
		transforms = append(transforms, transform.ReplaceMetricsServerResources(instance.Spec.MetricsServer.Resources, r.Scheme))
	}

	if !reflect.DeepEqual(instance.Spec.MetricsServer.AuditConfig, kedav1alpha1.AuditConfig{}) {
		transforms = auditConfigTransformation(transforms, instance.Spec.MetricsServer.AuditConfig, r.Scheme, logger)
	}

	// add arbitrary args defined by user
	for i := range instance.Spec.MetricsServer.Args {
		i := i
		transforms = append(transforms, transform.ReplaceArbitraryArg(instance.Spec.MetricsServer.Args[i], "metricsserver", r.Scheme, logger))
	}

	// replace namespace in RoleBinding from keda to kube-system
	transforms = append(transforms, transform.ReplaceNamespace(roleBindingName, roleBindingNamespace, r.Scheme, logger))

	manifest, err := r.resourcesMetrics.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform Metrics Server manifest")
		return err
	}
	r.resourcesMetrics = manifest

	if err := r.resourcesMetrics.Apply(); err != nil {
		logger.Error(err, "Unable to install Metrics Server")
		return err
	}

	return nil
}

func (r *KedaControllerReconciler) ensureOpenshiftCABundleConfigMap(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Ensure ConfigMap for OpenShift CA bundle exists")

	configMap := &corev1.ConfigMap{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: caBundleConfigMapName, Namespace: instance.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			configMap.Name = caBundleConfigMapName
			configMap.Namespace = instance.Namespace
			metav1.SetMetaDataAnnotation(&configMap.ObjectMeta, injectCABundleAnnotation, injectCABundleAnnotationValue)

			if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
				logger.Error(err, "Failed to set Controller Reference for ConfigMap")
				return err
			}

			err = r.Client.Create(ctx, configMap)
			if err != nil {
				logger.Error(err, "Failed to create new ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", caBundleConfigMapName)
				return err
			}

			return nil
		}
		// Error reading the object
		logger.Error(err, "Failed to get ConfigMap from cluster")
		return err
	}

	configMapUpdate := false

	if !metav1.HasAnnotation(configMap.ObjectMeta, injectCABundleAnnotation) ||
		configMap.Annotations[injectCABundleAnnotation] != injectCABundleAnnotationValue {
		metav1.SetMetaDataAnnotation(&configMap.ObjectMeta, injectCABundleAnnotation, injectCABundleAnnotationValue)
		configMapUpdate = true
	}

	if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
		if !goerrors.Is(err, &controllerutil.AlreadyOwnedError{}) {
			logger.Error(err, "Failed to check Controller Reference for ConfigMap")
			return err
		}
	} else {
		configMapUpdate = true
	}

	if configMapUpdate {
		err = r.Client.Update(ctx, configMap)
		if err != nil {
			logger.Error(err, "Failed to update ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", caBundleConfigMapName)
			return err
		}
	}

	return nil
}

func (r *KedaControllerReconciler) ensureMetricsServerAuditLogPolicyConfigMap(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	policy := instance.Spec.MetricsServer.AuditConfig.Policy

	// create real policy struct from higher level wrapper AuditPolicy exposed to user
	realPolicy := auditv1.Policy{}
	realPolicy.APIVersion = "audit.k8s.io/v1"
	realPolicy.Kind = "Policy"
	realPolicy.Rules = policy.Rules
	realPolicy.OmitStages = policy.OmitStages
	realPolicy.OmitManagedFields = policy.OmitManagedFields
	dataBytes, err := yaml.Marshal(realPolicy)
	if err != nil {
		logger.Error(err, "failed to Marshal Auditlog Policy struct")
		return err
	}

	configMap := &corev1.ConfigMap{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: auditlogPolicyConfigMap, Namespace: instance.Namespace}, configMap)
	if err != nil {
		if errors.IsNotFound(err) {
			// create ConfigMap if not found
			configMap.Name = auditlogPolicyConfigMap
			configMap.Namespace = instance.Namespace
			configMap.Data = make(map[string]string)

			configMap.Data[auditPolicyFile] = string(dataBytes)

			if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
				logger.Error(err, "failed to set Controller Reference for Auditlog Policy ConfigMap")
				return err
			}

			err = r.Client.Create(ctx, configMap)
			if err != nil {
				logger.Error(err, "failed to create new Audit Policy ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", auditlogPolicyConfigMap)
				return err
			}
			return nil
		}
		// Error reading the object
		logger.Error(err, "failed to get ConfigMap from cluster")
		return err
	}

	configMapUpdate := false
	if configMap.Data[auditPolicyFile] != string(dataBytes) {
		configMapUpdate = true
		configMap.Data[auditPolicyFile] = string(dataBytes)
	}

	if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
		if !goerrors.Is(err, &controllerutil.AlreadyOwnedError{}) {
			logger.Error(err, "failed to check Controller Reference for ConfigMap")
			return err
		}
	} else {
		configMapUpdate = true
	}

	if configMapUpdate {
		err = r.Client.Update(ctx, configMap)
		if err != nil {
			logger.Error(err, "failed to update ConfigMap in cluster", "ConfigMap.Namespace", instance.Namespace, "ConfigMap.Name", auditlogPolicyConfigMap)
			return err
		}
	}
	return nil
}

func (r *KedaControllerReconciler) installAdmissionWebhooks(ctx context.Context, logger logr.Logger, instance *kedav1alpha1.KedaController) error {
	logger.Info("Reconciling KEDA Admission Webhooks deployment")
	transforms := []mf.Transformer{
		transform.InjectOwner(instance),
		transform.ReplaceAllNamespaces(instance.Namespace),
		transform.ReplaceWatchNamespace(instance.Spec.WatchNamespace, "keda-admission-webhooks", r.Scheme, logger),
	}

	// on OpenShift 4.10 (kube 1.23) and earlier, the RuntimeDefault SeccompProfile won't validate against any SCC
	if util.RunningOnOpenshift(ctx, logger, r.Client) && util.RunningOnClusterWithoutSeccompProfileDefault(logger, r.discoveryClient) {
		transforms = append(transforms, transform.RemoveSeccompProfileFromAdmissionWebhooks(r.Scheme, logger))
	}

	// certificates rotation works only on Openshift due to openshift/service-ca-operator
	if util.RunningOnOpenshift(ctx, logger, r.Client) {
		serviceName := "keda-admission-webhooks"
		certsSecretName := serviceName + "-certs"

		transforms = append(
			transforms, transform.EnsureCABundleInjectionForValidatingWebhookConfiguration(injectCABundleAnnotation, injectCABundleAnnotationValue, r.Scheme),
			transform.EnsureCertInjectionForService(serviceName, servingCertsAnnotation, certsSecretName),
			transform.AdmissionWebhooksEnsureCertificatesVolume(caBundleConfigMapName, certsSecretName, r.Scheme),
		)
	}

	// Use alternate image spec if env var set
	if controllerImage := os.Getenv("KEDA_ADMISSION_WEBHOOKS_IMAGE"); len(controllerImage) > 0 {
		transforms = append(transforms, transform.ReplaceAdmissionWebhooksImage(controllerImage, r.Scheme))
	}

	if len(instance.Spec.AdmissionWebhooks.LogLevel) > 0 {
		transforms = append(transforms, transform.ReplaceAdmissionWebhooksLogLevel(instance.Spec.AdmissionWebhooks.LogLevel, r.Scheme, logger))
	}
	if len(instance.Spec.AdmissionWebhooks.LogEncoder) > 0 {
		transforms = append(transforms, transform.ReplaceAdmissionWebhooksLogEncoder(instance.Spec.AdmissionWebhooks.LogEncoder, r.Scheme, logger))
	}
	if len(instance.Spec.AdmissionWebhooks.LogTimeEncoding) > 0 {
		transforms = append(transforms, transform.ReplaceAdmissionWebhooksLogTimeEncoding(instance.Spec.AdmissionWebhooks.LogTimeEncoding, r.Scheme, logger))
	}

	if len(instance.Spec.AdmissionWebhooks.DeploymentAnnotations) > 0 {
		transforms = append(transforms, transform.AddDeploymentAnnotations(instance.Spec.AdmissionWebhooks.DeploymentAnnotations, r.Scheme))
	}

	if len(instance.Spec.AdmissionWebhooks.DeploymentLabels) > 0 {
		transforms = append(transforms, transform.AddDeploymentLabels(instance.Spec.AdmissionWebhooks.DeploymentLabels, r.Scheme))
	}

	if len(instance.Spec.AdmissionWebhooks.PodAnnotations) > 0 {
		transforms = append(transforms, transform.AddPodAnnotations(instance.Spec.AdmissionWebhooks.PodAnnotations, r.Scheme))
	}

	if len(instance.Spec.AdmissionWebhooks.PodLabels) > 0 {
		transforms = append(transforms, transform.AddPodLabels(instance.Spec.AdmissionWebhooks.PodLabels, r.Scheme))
	}

	if len(instance.Spec.AdmissionWebhooks.NodeSelector) > 0 {
		transforms = append(transforms, transform.ReplaceNodeSelector(instance.Spec.AdmissionWebhooks.NodeSelector, r.Scheme))
	}

	if len(instance.Spec.AdmissionWebhooks.Tolerations) > 0 {
		transforms = append(transforms, transform.ReplaceTolerations(instance.Spec.AdmissionWebhooks.Tolerations, r.Scheme))
	}

	if instance.Spec.AdmissionWebhooks.Affinity != nil {
		transforms = append(transforms, transform.ReplaceAffinity(instance.Spec.AdmissionWebhooks.Affinity, r.Scheme))
	}

	if len(instance.Spec.AdmissionWebhooks.PriorityClassName) > 0 {
		transforms = append(transforms, transform.ReplacePriorityClassName(instance.Spec.AdmissionWebhooks.PriorityClassName, r.Scheme))
	}

	if instance.Spec.AdmissionWebhooks.Resources.Limits != nil || instance.Spec.AdmissionWebhooks.Resources.Requests != nil {
		transforms = append(transforms, transform.ReplaceAdmissionWebhooksResources(instance.Spec.AdmissionWebhooks.Resources, r.Scheme))
	}

	// add arbitrary args defined by user
	for i := range instance.Spec.AdmissionWebhooks.Args {
		i := i
		transforms = append(transforms, transform.ReplaceArbitraryArg(instance.Spec.AdmissionWebhooks.Args[i], "admissionwebhooks", r.Scheme, logger))
	}

	manifest, err := r.resourcesWebhooks.Transform(transforms...)
	if err != nil {
		logger.Error(err, "Unable to transform KEDA Admission Webhooks manifest")
		return err
	}
	r.resourcesWebhooks = manifest

	if err := r.resourcesWebhooks.Apply(); err != nil {
		logger.Error(err, "Unable to install KEDA Admission Webhooks")
		return err
	}

	return nil
}

// Because it's effectively cluster-scoped, we only care about a
// single, named resource: keda in the specified namespace
func isInteresting(request reconcile.Request, kedaControllerResourceNamespace string) bool {
	return request.Name == kedaControllerResourceName && request.Namespace == kedaControllerResourceNamespace
}

// auditConfigTransformation is support function for transforming basic audit
// flags. audit-log-path and audit-policy-file are transformed separately because of
// different complexity. Returns transforms ([]mf.Transformer type) after all
// flags are appended
func auditConfigTransformation(t []mf.Transformer, ac kedav1alpha1.AuditConfig, scheme *runtime.Scheme, logger logr.Logger) []mf.Transformer {
	if ac.LogFormat != "" {
		t = append(t, transform.ReplaceAuditConfig(ac.LogFormat, "logformat", scheme, logger))
	}
	if ac.AuditLifetime.MaxAge != "" {
		t = append(t, transform.ReplaceAuditConfig(ac.AuditLifetime.MaxAge, "maxage", scheme, logger))
	}
	if ac.AuditLifetime.MaxBackup != "" {
		t = append(t, transform.ReplaceAuditConfig(ac.AuditLifetime.MaxBackup, "maxbackup", scheme, logger))
	}
	if ac.AuditLifetime.MaxSize != "" {
		t = append(t, transform.ReplaceAuditConfig(ac.AuditLifetime.MaxSize, "maxsize", scheme, logger))
	}
	return t
}

// validateAuditLogVolumeWithArgs validates whether Volume can exist with lifetime
// arguments. If name is empty (no volume given) -> validate lifetime args are
// not given otherwise lt args would be useless for stdout logging.
func validateAuditLogVolumeWithArgs(name string, ltArgs kedav1alpha1.AuditLifetime) error {
	if name == "" {
		var maxage int
		var maxbackup int
		var maxsize int
		var err error

		// check if lifetime args are given -> this would be error
		if ltArgs.MaxAge != "" {
			maxage, err = strconv.Atoi(ltArgs.MaxAge)
			if err != nil {
				return fmt.Errorf("bad conversion of string in audit flag maxAge")
			}
		}
		if ltArgs.MaxBackup != "" {
			maxbackup, err = strconv.Atoi(ltArgs.MaxBackup)
			if err != nil {
				return fmt.Errorf("bad conversion of string in audit flag maxBackup")
			}
		}
		if ltArgs.MaxSize != "" {
			maxsize, err = strconv.Atoi(ltArgs.MaxSize)
			if err != nil {
				return fmt.Errorf("bad conversion of string in audit flag maxSize")
			}
		}
		if maxage >= 1 || maxbackup >= 1 || maxsize >= 1 {
			return fmt.Errorf("bad flag combination - don't use lifetime arguments when logging to stdout")
		}
	}
	return nil
}

// checkAuditLogVolumeExists checks whether PersistentVolumeClaim given exists
// and is bound to a PV or not
func (r *KedaControllerReconciler) checkAuditLogVolumeExists(name string, ctx context.Context, instance *kedav1alpha1.KedaController) error {
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, pvc)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("persistentVolumeClaim '%s not found in namespace %s", name, instance.Namespace)
		}
	}

	return nil
}
