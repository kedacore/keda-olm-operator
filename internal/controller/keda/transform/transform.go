package transform

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	mf "github.com/manifestival/manifestival"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

var (
	logLevels        = []string{"debug", "info", "error"}
	logEncoders      = []string{"json", "console"}
	logTimeEncodings = []string{"epoch", "millis", "nano", "iso8601", "rfc3339", "rfc3339nano"}
)

type Prefix string

const (
	LogLevelArg           Prefix = "--zap-log-level="
	LogEncoderArg         Prefix = "--zap-encoder="
	LogTimeEncodingArg    Prefix = "--zap-time-encoding="
	LogLevelMetricsServer Prefix = "--v="
	ClientCAFile          Prefix = "--client-ca-file="
	TLSCertFile           Prefix = "--tls-cert-file="
	TLSPrivateKeyFile     Prefix = "--tls-private-key-file="
	CertRotation          Prefix = "--enable-cert-rotation="
	CADir                 Prefix = "--ca-dir="
)

func (p Prefix) String() string {
	return string(p)
}

const (
	defaultNamespace               = "keda"
	containerNameKedaOperator      = "keda-operator"
	containerNameMetricsServer     = "keda-metrics-apiserver"
	containerNameAdmissionWebhooks = "keda-admission-webhooks"
	caCertVolPrefix                = "cabundle"
)

// ReplaceAllNamespaces returns a transformer which will traverse the unstructured content looking for map entries with
// (key, value) of ("namespace", "keda"), changing the value of any matches to namespace
func ReplaceAllNamespaces(namespace string) mf.Transformer {
	// recursive function helper
	var helper func(obj interface{})

	helper = func(obj interface{}) {
		if obj == nil || reflect.ValueOf(obj).Kind() == reflect.Pointer && reflect.ValueOf(obj).IsNil() {
			return
		}
		if mapInstance, ok := obj.(map[string]interface{}); ok {
			for k, v := range mapInstance {
				if stringVal, ok := v.(string); k == "namespace" && ok && stringVal == defaultNamespace {
					mapInstance[k] = namespace
				} else {
					helper(v)
				}
			}
		} else if arrayInstance, ok := obj.([]interface{}); ok {
			for _, item := range arrayInstance {
				helper(item)
			}
		} else {
			return
		}
	}

	return func(u *unstructured.Unstructured) error {
		helper(u.UnstructuredContent())
		return nil
	}
}

func ReplaceNamespace(name string, namespace string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetName() == name {
			logger.Info("Changing namespace to " + namespace)

			rolebinding := &rbacv1.RoleBinding{}
			if err := scheme.Convert(u, rolebinding, nil); err != nil {
				return err
			}

			rolebinding.Namespace = namespace

			if err := scheme.Convert(rolebinding, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func ReplaceWatchNamespace(watchNamespace string, containerName string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		changed := false
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					for j, env := range container.Env {
						if env.Name == "WATCH_NAMESPACE" {
							if env.Value != watchNamespace {
								logger.Info("Replacing", "deployment", container.Name, "WATCH_NAMESPACE", watchNamespace, "previous", env.Value)
								containers[i].Env[j].Value = watchNamespace
								changed = true
							}
							break
						}
					}
					break
				}
			}
			if changed {
				if err := scheme.Convert(deploy, u, nil); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func RemoveSeccompProfile(containerName string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		changed := false
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					if container.SecurityContext != nil && container.SecurityContext.SeccompProfile != nil && container.SecurityContext.SeccompProfile.Type == corev1.SeccompProfileTypeRuntimeDefault {
						containers[i].SecurityContext.SeccompProfile = nil
						changed = true
						logger.Info("Removed SeccomProfile from SecurityContext from pod", "container", container.Name)
						break
					}
				}
			}
			if changed {
				if err := scheme.Convert(deploy, u, nil); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func RemoveSeccompProfileFromKedaOperator(scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return RemoveSeccompProfile(containerNameKedaOperator, scheme, logger)
}

func RemoveSeccompProfileFromMetricsServer(scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return RemoveSeccompProfile(containerNameMetricsServer, scheme, logger)
}

func RemoveSeccompProfileFromAdmissionWebhooks(scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return RemoveSeccompProfile(containerNameAdmissionWebhooks, scheme, logger)
}

func EnsureCABundleInjectionForValidatingWebhookConfiguration(annotation string, annotationValue string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ValidatingWebhookConfiguration" {
			vwc := &admissionregistrationv1.ValidatingWebhookConfiguration{}
			if err := scheme.Convert(u, vwc, nil); err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&vwc.ObjectMeta, annotation, annotationValue)

			if err := scheme.Convert(vwc, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func EnsureCABundleInjectionForAPIService(annotation string, annotationValue string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "APIService" {
			apiService := &apiregistrationv1.APIService{}
			if err := scheme.Convert(u, apiService, nil); err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&apiService.ObjectMeta, annotation, annotationValue)
			apiService.Spec.InsecureSkipTLSVerify = false

			if err := scheme.Convert(apiService, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func EnsureCertInjectionForService(serviceName string, annotation string, annotationValue string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Service" && u.GetName() == serviceName {
			annotations := u.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			annotations[annotation] = annotationValue
			u.SetAnnotations(annotations)
		}
		return nil
	}
}

func MetricsServerEnsureCertificatesVolume(configMapName, secretName string, scheme *runtime.Scheme) mf.Transformer {
	return ensureCertificatesVolumeForDeployment(containerNameMetricsServer, configMapName, secretName, scheme)
}

func KedaOperatorEnsureCertificatesVolume(serviceSecretName string, grpcClientCertsSecretName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			certificatesVolume := corev1.Volume{
				Name: "certificates",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: serviceSecretName,
									},
									Items: []corev1.KeyToPath{
										{Key: "tls.crt", Path: "tls.crt"}, // use OpenShift-generated service cert for gRPC service
										{Key: "tls.key", Path: "tls.key"},
									},
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: grpcClientCertsSecretName,
									},
									Items: []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}}, // trust clients using kedaorg-certs cert
								},
							},
						},
					},
				},
			}

			volumes := deploy.Spec.Template.Spec.Volumes
			certificatesVolumeFound := false
			for i := range volumes {
				if volumes[i].Name == "certificates" {
					volumes[i] = certificatesVolume
					certificatesVolumeFound = true
				}
			}

			if !certificatesVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, certificatesVolume)
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i := range containers {
				if containers[i].Name == containerNameAdmissionWebhooks {
					// mount Volume referencing certs in Secrets
					certificatesVolumeMount := corev1.VolumeMount{
						Name:      "certificates",
						MountPath: "/certs",
						ReadOnly:  true,
					}

					volumeMounts := containers[i].VolumeMounts
					certificatesVolumeMountFound := false
					for j := range volumeMounts {
						if volumeMounts[j].Name == "certificates" {
							volumeMounts[j] = certificatesVolumeMount
							certificatesVolumeMountFound = true
						}
					}

					if !certificatesVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, certificatesVolumeMount)
					}

					break
				}
			}

			if err := scheme.Convert(deploy, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func AdmissionWebhooksEnsureCertificatesVolume(configMapName, secretName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			certificatesVolume := corev1.Volume{
				Name: "certificates",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretName,
									},
								},
							},
							{
								ConfigMap: &corev1.ConfigMapProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
									Items: []corev1.KeyToPath{{Key: "service-ca.crt", Path: "ca.crt"}},
								},
							},
						},
					},
				},
			}

			volumes := deploy.Spec.Template.Spec.Volumes
			certificatesVolumeFound := false
			for i := range volumes {
				if volumes[i].Name == "certificates" {
					volumes[i] = certificatesVolume
					certificatesVolumeFound = true
				}
			}

			if !certificatesVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, certificatesVolume)
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i := range containers {
				if containers[i].Name == containerNameAdmissionWebhooks {
					// mount Volume referencing certs in ConfigMap and Secret
					certificatesVolumeMount := corev1.VolumeMount{
						Name:      "certificates",
						MountPath: "/certs",
						ReadOnly:  true,
					}

					volumeMounts := containers[i].VolumeMounts
					certificatesVolumeMountFound := false
					for j := range volumeMounts {
						if volumeMounts[j].Name == "certificates" {
							volumeMounts[j] = certificatesVolumeMount
							certificatesVolumeMountFound = true
						}
					}

					if !certificatesVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, certificatesVolumeMount)
					}

					break
				}
			}

			if err := scheme.Convert(deploy, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func ensureCertificatesVolumeForDeployment(containerName, configMapName, secretName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			// add Volumes referencing certs in ConfigMap and Secret
			var cabundleVolume corev1.Volume
			if containerName == containerNameKedaOperator {
				cabundleVolume = corev1.Volume{
					Name: caCertVolPrefix,
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
						},
					},
				}
			}

			certificatesVolume := corev1.Volume{
				Name: "certificates",
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "kedaorg-certs",
									},
									Items: []corev1.KeyToPath{
										{Key: "tls.crt", Path: "tls.crt"}, // use the generated gRPC client cert
										{Key: "tls.key", Path: "tls.key"},
									},
								},
							},
							{
								Secret: &corev1.SecretProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: secretName,
									},
									Items: []corev1.KeyToPath{
										{Key: "tls.crt", Path: "ocp-tls.crt"},
										{Key: "tls.key", Path: "ocp-tls.key"},
									},
								},
							},
							{
								ConfigMap: &corev1.ConfigMapProjection{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
									Items: []corev1.KeyToPath{{Key: "service-ca.crt", Path: "ca.crt"}}, // trust openshift-generated service certs
								},
							},
						},
					},
				},
			}

			volumes := deploy.Spec.Template.Spec.Volumes
			cabundleVolumeFound := false
			certificatesVolumeFound := false
			for i := range volumes {
				if containerName == containerNameKedaOperator {
					if volumes[i].Name == caCertVolPrefix {
						volumes[i] = cabundleVolume
						cabundleVolumeFound = true
					}
				}
				if volumes[i].Name == "certificates" {
					volumes[i] = certificatesVolume
					certificatesVolumeFound = true
				}
			}

			if containerName == containerNameKedaOperator && !cabundleVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, cabundleVolume)
			}
			if !certificatesVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, certificatesVolume)
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i := range containers {
				if containers[i].Name == containerName {
					var cabundleVolumeMount corev1.VolumeMount
					if containerName == containerNameKedaOperator {
						cabundleVolumeMount = corev1.VolumeMount{
							Name:      caCertVolPrefix,
							MountPath: "/custom/ca",
						}
					}

					// mount Volume referencing certs in ConfigMap and Secret
					certificatesVolumeMount := corev1.VolumeMount{
						Name:      "certificates",
						MountPath: "/certs",
						ReadOnly:  true,
					}

					volumeMounts := containers[i].VolumeMounts
					cabundleVolumeMountFound := false
					certificatesVolumeMountFound := false
					for j := range volumeMounts {
						// add custom CA to Operator
						if containerName == containerNameKedaOperator {
							if volumeMounts[j].Name == caCertVolPrefix {
								volumeMounts[j] = cabundleVolumeMount
								cabundleVolumeMountFound = true
							}
						}
						if volumeMounts[j].Name == "certificates" {
							volumeMounts[j] = certificatesVolumeMount
							certificatesVolumeMountFound = true
						}
					}

					if containerName == containerNameKedaOperator && !cabundleVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, cabundleVolumeMount)
					}
					if !certificatesVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, certificatesVolumeMount)
					}

					break
				}
			}

			if err := scheme.Convert(deploy, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

// Add configmap volumes for configMapNames named cabundle0, cabundle1, etc. as /custom/ca0, /custom/ca1, etc. with
// container args --ca-dir=/custom/ca0, --ca-dir=/custom/ca1, etc.
func EnsureCACertsForOperatorDeployment(configMapNames []string, scheme *runtime.Scheme, logger logr.Logger) []mf.Transformer {
	var retval []mf.Transformer

	var caDirs []string
	for i := range configMapNames {
		caDirs = append(caDirs, "/custom/ca"+strconv.Itoa(i))
	}
	retval = append(retval, replaceContainerArgs(caDirs, CADir, containerNameKedaOperator, scheme, logger))

	retval = append(retval, func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			// add Volumes referencing certs in ConfigMap
			cabundleVolumes := map[string]corev1.Volume{}
			cabundleVolumeMounts := map[string]corev1.VolumeMount{}
			for i, configMapName := range configMapNames {
				cabundleVolumes[configMapName] = corev1.Volume{
					Name: caCertVolPrefix + strconv.Itoa(i),
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: configMapName,
							},
						},
					},
				}

				cabundleVolumeMounts[caCertVolPrefix+strconv.Itoa(i)] = corev1.VolumeMount{
					Name:      caCertVolPrefix + strconv.Itoa(i),
					MountPath: "/custom/ca" + strconv.Itoa(i),
				}
			}

			cabundleVolumeFound := map[string]bool{}
			var volumes []corev1.Volume
			for _, vol := range deploy.Spec.Template.Spec.Volumes {
				if caVol, ok := cabundleVolumes[vol.Name]; ok {
					volumes = append(volumes, caVol)
					cabundleVolumeFound[vol.Name] = true
				} else if !strings.HasPrefix(vol.Name, caCertVolPrefix) {
					volumes = append(volumes, vol)
				} // else don't copy it over since it shouldn't be there
			}

			for name, vol := range cabundleVolumes {
				if !cabundleVolumeFound[name] {
					volumes = append(volumes, vol)
				}
			}
			deploy.Spec.Template.Spec.Volumes = volumes

			containers := deploy.Spec.Template.Spec.Containers
			for i := range containers {
				if containers[i].Name == containerNameKedaOperator {
					// mount Volumes referencing certs in ConfigMap
					var volumeMounts []corev1.VolumeMount
					cabundleVolumeMountFound := map[string]bool{}
					for _, volMount := range containers[i].VolumeMounts {
						if caVolMount, ok := cabundleVolumeMounts[volMount.Name]; ok {
							volumeMounts = append(volumeMounts, caVolMount)
							cabundleVolumeMountFound[volMount.Name] = true
						} else if !strings.HasPrefix(volMount.Name, caCertVolPrefix) {
							volumeMounts = append(volumeMounts, volMount)
						} // else don't copy it over since it shouldn't be there
					}
					for name, volmount := range cabundleVolumeMounts {
						if !cabundleVolumeMountFound[name] {
							volumeMounts = append(volumeMounts, volmount)
						}
					}
					containers[i].VolumeMounts = volumeMounts
					break
				}
			}

			if err := scheme.Convert(deploy, u, nil); err != nil {
				return err
			}
		}
		return nil
	})
	return retval
}

func EnsurePathsToCertsInDeployment(values []string, prefixes []Prefix, scheme *runtime.Scheme, logger logr.Logger) []mf.Transformer {
	transforms := []mf.Transformer{}
	for i := range values {
		transforms = append(transforms, replaceContainerArg(values[i], prefixes[i], containerNameMetricsServer, scheme, logger))
	}
	return transforms
}

func EnsureAuditPolicyConfigMapMountsVolume(configMapName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			policyVolume := corev1.Volume{
				Name: "audit-policy",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			}
			volumes := deploy.Spec.Template.Spec.Volumes
			policyVolumeFound := false
			for i := range volumes {
				if volumes[i].Name == "audit-policy" {
					volumes[i] = policyVolume
					policyVolumeFound = true
				}
			}

			// add Volume to deployment if not found
			if !policyVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, policyVolume)
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i := range containers {
				if containers[i].Name == containerNameMetricsServer {
					policyVolumeMount := corev1.VolumeMount{
						Name:      "audit-policy",
						MountPath: "/var/audit-policy",
					}

					volumeMounts := containers[i].VolumeMounts
					policyVolumeMountFound := false
					for j := range volumeMounts {
						if volumeMounts[j].Name == "audit-policy" {
							volumeMounts[j] = policyVolumeMount
							policyVolumeMountFound = true
						}
					}
					// add VolumeMount to deployment if not found
					if !policyVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, policyVolumeMount)
					}

					break
				}
			}
			if err := scheme.Convert(deploy, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func ReplaceKedaOperatorLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	found := false
	for _, level := range logLevels {
		if logLevel == level {
			found = true
		}
	}
	if !found {
		if _, err := strconv.ParseUint(logLevel, 10, 64); err == nil {
			found = true
		}
	}

	if !found {
		logger.Info("Ignoring speficied Log level for KEDA Operator, it needs to be set to ", strings.Join(logLevels, ", "), "or an integer value greater than 0")
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}

	prefix := LogLevelArg
	return replaceContainerArg(logLevel, prefix, containerNameKedaOperator, scheme, logger)
}

func ReplaceKedaOperatorLogEncoder(logEncoder string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	found := false
	for _, format := range logEncoders {
		if logEncoder == format {
			found = true
			break
		}
	}

	if !found {
		logger.Info("Ignoring speficied Log encoder for KEDA Operator", "specified", logEncoder, "allowed values", strings.Join(logEncoders, ", "))
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}

	prefix := LogEncoderArg
	return replaceContainerArg(logEncoder, prefix, containerNameKedaOperator, scheme, logger)
}

func ReplaceMetricsServerLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	found := false
	if _, err := strconv.ParseUint(logLevel, 10, 64); err == nil {
		found = true
	}

	if !found {
		logger.Info("Ignoring speficied Log level for KEDA Metrics Server, it needs to be set to an integer value greater than 0")
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}

	prefix := LogLevelMetricsServer
	return replaceContainerArg(logLevel, prefix, containerNameMetricsServer, scheme, logger)
}

func ReplaceKedaOperatorLogTimeEncoding(logTimeEncoding string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	found := false
	for _, timeEncoding := range logTimeEncodings {
		if logTimeEncoding == timeEncoding {
			found = true
			break
		}
	}

	if !found {
		logger.Info("Ignoring speficied Log time encoding for KEDA Operator", "specified", logTimeEncoding, "allowed values", strings.Join(logTimeEncodings, ", "))
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}

	prefix := LogTimeEncodingArg
	return replaceContainerArg(logTimeEncoding, prefix, containerNameKedaOperator, scheme, logger)
}

func ReplaceAdmissionWebhooksLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	found := false
	for _, level := range logLevels {
		if logLevel == level {
			found = true
		}
	}
	if !found {
		if _, err := strconv.ParseUint(logLevel, 10, 64); err == nil {
			found = true
		}
	}

	if !found {
		logger.Info("Ignoring speficied Log level for KEDA Admission Webhooks, it needs to be set to ", strings.Join(logLevels, ", "), "or an integer value greater than 0")
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}

	prefix := LogLevelArg
	return replaceContainerArg(logLevel, prefix, containerNameAdmissionWebhooks, scheme, logger)
}

func ReplaceAdmissionWebhooksLogEncoder(logEncoder string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	found := false
	for _, format := range logEncoders {
		if logEncoder == format {
			found = true
			break
		}
	}

	if !found {
		logger.Info("Ignoring speficied Log encoder for KEDA Admission Webhooks", "specified", logEncoder, "allowed values", strings.Join(logEncoders, ", "))
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}

	prefix := LogEncoderArg
	return replaceContainerArg(logEncoder, prefix, containerNameAdmissionWebhooks, scheme, logger)
}

func ReplaceAdmissionWebhooksLogTimeEncoding(logTimeEncoding string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	found := false
	for _, timeEncoding := range logTimeEncodings {
		if logTimeEncoding == timeEncoding {
			found = true
			break
		}
	}

	if !found {
		logger.Info("Ignoring speficied Log time encoding for KEDA Admission Webhooks", "specified", logTimeEncoding, "allowed values", strings.Join(logTimeEncodings, ", "))
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}

	prefix := LogTimeEncodingArg
	return replaceContainerArg(logTimeEncoding, prefix, containerNameAdmissionWebhooks, scheme, logger)
}

func ReplaceArbitraryArg(argument string, resource string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	prefix := Prefix("")
	prefixStr := ""
	argTrue := ""

	if strings.Contains(argument, "=") {
		// if argument is in format --arg=value or arg=value
		stringSplit := strings.SplitAfter(argument, "=")
		if !strings.HasPrefix(stringSplit[0], "--") {
			// add "--" at the beginning of argument prefix if not given
			prefixStr = "--" + stringSplit[0]
		} else {
			prefixStr = stringSplit[0]
		}

		prefix = Prefix(prefixStr)
		argTrue = stringSplit[1]
	} else {
		// if argument is just value like '/usr/local/bin/keda-adapter' (has no "=" therefore no prefix)
		argTrue = argument
	}

	switch resource {
	case "operator":
		return replaceContainerArg(argTrue, prefix, containerNameKedaOperator, scheme, logger)
	case "metricsserver":
		return replaceContainerArg(argTrue, prefix, containerNameMetricsServer, scheme, logger)
	case "admissionwebhooks":
		return replaceContainerArg(argTrue, prefix, containerNameAdmissionWebhooks, scheme, logger)
	default:
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}
}

func SetOperatorCertRotation(enable bool, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	arg := "false"
	if enable {
		arg = "true"
	}
	return replaceContainerArg(arg, CertRotation, containerNameKedaOperator, scheme, logger)
}

func ReplaceAuditConfig(argument string, selector string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	var prefix string
	switch selector {
	case "policyfile":
		prefix = "--audit-policy-file="
	case "logformat":
		prefix = "--audit-log-format="
	case "logpath":
		prefix = "--audit-log-path="
	case "maxage":
		prefix = "--audit-log-maxage="
	case "maxbackup":
		prefix = "--audit-log-maxbackup="
	case "maxsize":
		prefix = "--audit-log-maxsize="
	default:
		return func(*unstructured.Unstructured) error {
			return nil
		}
	}
	return replaceContainerArg(argument, Prefix(prefix), containerNameMetricsServer, scheme, logger)
}

func replaceContainerArg(value string, prefix Prefix, containerName string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		changed := false
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					argFound := false
					for j, arg := range container.Args {
						if strings.HasPrefix(arg, prefix.String()) {
							// If argument has no prefix...
							if prefix == "" {
								// and is the same -> dont add it again (change argFound=true)...
								if arg == value {
									argFound = true
									break
								}
								// otherwise continue
								continue
							}

							argFound = true
							if trimmedArg := strings.TrimPrefix(arg, prefix.String()); trimmedArg != value {
								logger.Info("Replacing", "deployment", container.Name, "prefix", prefix.String(), "value", value, "previous", trimmedArg)
								containers[i].Args[j] = prefix.String() + value
								changed = true
							}
							break
						}
					}
					if !argFound {
						logger.Info("Adding", "deployment", container.Name, "prefix", prefix.String(), "value", value)
						containers[i].Args = append(containers[i].Args, prefix.String()+value)
						changed = true
					}
					break
				}
			}
			if changed {
				if err := scheme.Convert(deploy, u, nil); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func replaceContainerArgs(values []string, prefix Prefix, containerName string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// this function only supports flags with a prefix
		if prefix == "" {
			return nil
		}
		changed := false
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					argFound := false
					var newArgs []string
					for _, arg := range container.Args {
						if !strings.HasPrefix(arg, prefix.String()) {
							newArgs = append(newArgs, arg)
						} else {
							if argFound {
								continue
							}
							argFound = true
							for _, value := range values {
								newArgs = append(newArgs, prefix.String()+value)
							}
						}
					}
					if argFound {
						changed = !reflect.DeepEqual(containers[i].Args, newArgs)
						if changed {
							logger.Info("Updating args", "deployment", container.Name, "prefix", prefix.String(), "values", values)
							containers[i].Args = newArgs
						}
					} else if len(values) > 0 {
						logger.Info("Adding args", "deployment", container.Name, "prefix", prefix.String(), "value", values)
						for _, value := range values {
							containers[i].Args = append(containers[i].Args, prefix.String()+value)
						}
						changed = true
					}
					break
				}
			}
			if changed {
				if err := scheme.Convert(deploy, u, nil); err != nil {
					return err
				}
			}
		}
		return nil
	}
}

func AddServiceAccountAnnotations(annotations map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ServiceAccount" {
			sa := &corev1.ServiceAccount{}
			if err := scheme.Convert(u, sa, nil); err != nil {
				return err
			}

			sa.Annotations = updateMap(sa.Annotations, annotations)

			return scheme.Convert(sa, u, nil)
		}
		return nil
	}
}

func AddServiceAccountLabels(labels map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ServiceAccount" {
			sa := &corev1.ServiceAccount{}
			if err := scheme.Convert(u, sa, nil); err != nil {
				return err
			}

			sa.Labels = updateMap(sa.Labels, labels)

			return scheme.Convert(sa, u, nil)
		}
		return nil
	}
}

func AddPodAnnotations(annotations map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Spec.Template.Annotations = updateMap(deploy.Spec.Template.Annotations, annotations)

			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func AddPodLabels(labels map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Spec.Template.ObjectMeta.Labels = updateMap(deploy.Spec.Template.ObjectMeta.Labels, labels)

			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func AddDeploymentAnnotations(annotations map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Annotations = updateMap(deploy.Annotations, annotations)

			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func AddDeploymentLabels(labels map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Labels = updateMap(deploy.Labels, labels)

			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func updateMap(mapToUpdate map[string]string, newValues map[string]string) map[string]string {
	if mapToUpdate != nil {
		for k, v := range newValues {
			mapToUpdate[k] = v
		}
		return mapToUpdate
	}

	return newValues
}

func ReplaceNodeSelector(nodeSelector map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Spec.Template.Spec.NodeSelector = nodeSelector
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func ReplaceTolerations(tolerations []corev1.Toleration, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Spec.Template.Spec.Tolerations = tolerations
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func ReplaceAffinity(affinity *corev1.Affinity, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Spec.Template.Spec.Affinity = affinity
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func ReplacePriorityClassName(priorityClassName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			deploy.Spec.Template.Spec.PriorityClassName = priorityClassName
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func ReplaceKedaOperatorResources(resources corev1.ResourceRequirements, scheme *runtime.Scheme) mf.Transformer {
	return replaceResources(resources, containerNameKedaOperator, scheme)
}

func ReplaceMetricsServerResources(resources corev1.ResourceRequirements, scheme *runtime.Scheme) mf.Transformer {
	return replaceResources(resources, containerNameMetricsServer, scheme)
}

func ReplaceAdmissionWebhooksResources(resources corev1.ResourceRequirements, scheme *runtime.Scheme) mf.Transformer {
	return replaceResources(resources, containerNameAdmissionWebhooks, scheme)
}

func replaceResources(resources corev1.ResourceRequirements, containerName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					containers[i].Resources = resources
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func ReplaceMetricsServerImage(image string, scheme *runtime.Scheme) mf.Transformer {
	return replaceContainerImage(image, containerNameMetricsServer, scheme)
}

func ReplaceKedaOperatorImage(image string, scheme *runtime.Scheme) mf.Transformer {
	return replaceContainerImage(image, containerNameKedaOperator, scheme)
}

func ReplaceAdmissionWebhooksImage(image string, scheme *runtime.Scheme) mf.Transformer {
	return replaceContainerImage(image, containerNameAdmissionWebhooks, scheme)
}

func replaceContainerImage(image string, containerName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					containers[i].Image = image
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func EnsureAuditLogMount(pvc string, path string, scheme *runtime.Scheme) mf.Transformer {
	const logOutputVolumeName = "audit-log"
	return func(u *unstructured.Unstructured) error {
		// ensure mountVolume exists when volume exists
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			logOutVolume := corev1.Volume{
				Name: logOutputVolumeName,
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: pvc,
					},
				},
			}
			// find volume by name
			volumes := deploy.Spec.Template.Spec.Volumes
			logVolumeFound := false
			for i := range volumes {
				if volumes[i].Name == logOutputVolumeName {
					volumes[i] = logOutVolume
					logVolumeFound = true
				}
			}

			// if not found
			if !logVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, logOutVolume)
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i := range containers {
				if containers[i].Name == containerNameMetricsServer {
					auditVolumeMount := corev1.VolumeMount{
						Name:      logOutputVolumeName,
						MountPath: path,
					}

					volumeMounts := containers[i].VolumeMounts
					auditLogVolumeMountFound := false
					for j := range volumeMounts {
						if volumeMounts[j].Name == logOutputVolumeName {
							volumeMounts[j] = auditVolumeMount
							auditLogVolumeMountFound = true
						}
					}
					// add VolumeMount to deployment if not found
					if !auditLogVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, auditVolumeMount)
					}
					break
				}
			}
			if err := scheme.Convert(deploy, u, nil); err != nil {
				return err
			}
		}
		return nil
	}
}

func ReplaceDeploymentVolumes(desiredVolumes []corev1.Volume, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			volumes := deploy.Spec.Template.Spec.Volumes

			for j, volumeDevice := range desiredVolumes {
				// for each existing volume mount
				var alreadyExists bool
				for k, existingVolumeDevice := range volumes {
					// if the existing volume mount is for the same volume, replace it
					if existingVolumeDevice.Name == volumeDevice.Name {
						alreadyExists = true
						deploy.Spec.Template.Spec.Volumes[k] = desiredVolumes[j]
						break
					}
				}
				// if we didn't find the name in our list already, add it (we allow both add and override)
				if !alreadyExists {
					deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, desiredVolumes[j])
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

func ReplaceDeploymentVolumeMounts(desiredVolumeMounts []corev1.VolumeMount, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			containers := deploy.Spec.Template.Spec.Containers
			// for each container
			for i := range containers {
				// only manipulate our operand containers, we don't want to attach these to all sidecars
				if containers[i].Name != containerNameKedaOperator &&
					containers[i].Name != containerNameMetricsServer &&
					containers[i].Name != containerNameAdmissionWebhooks {
					continue
				}
				// for each existing volume mount
				for j, volumeMount := range desiredVolumeMounts {
					var alreadyExists bool
					for k, existingVolumeMount := range containers[i].VolumeMounts {
						// if the existing volume mount is for the same volume, replace it
						if existingVolumeMount.Name == volumeMount.Name {
							alreadyExists = true
							containers[i].VolumeMounts[k] = volumeMount
							break
						}
					}
					// if we didn't find the name in our list already, add it (we allow both add and override)
					if !alreadyExists {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, desiredVolumeMounts[j])
					}
				}
				containers[i].VolumeMounts = desiredVolumeMounts
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// InjectOwner creates a Transformer which adds an OwnerReference
// pointing to `owner`, but only if the object is in the same namespace as `owner`
func InjectOwner(owner mf.Owner) mf.Transformer {
	f := mf.InjectOwner(owner) // This is just a wrapper around manifestival.InjectOwner
	return func(u *unstructured.Unstructured) error {
		// Since the controller is namespaced, it can only have namespace-scoped dependants in the same namespace
		// https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/
		if u.GetNamespace() == owner.GetNamespace() {
			return f(u)
		}
		return nil
	}
}
