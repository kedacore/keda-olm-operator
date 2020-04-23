package transform

import (
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	mf "github.com/jcrossley3/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1beta1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

var (
	logLevelsKedaOperator      = []string{"debug", "info", "error"}
	logTimeFormatsKedaOperator = []string{"epoch", "millis", "nano", "iso8601"}
)

type Prefix string

const (
	LogLevelKedaOperator      Prefix = "--zap-level="
	LogTimeFormatKedaOperator        = "--zap-time-encoding="
	LogLevelMetricsServer            = "--v="
	ClientCAFile                     = "--client-ca-file="
	TLSCertFile                      = "--tls-cert-file="
	TLSPrivateKeyFile                = "--tls-private-key-file="
)

func (p Prefix) String() string {
	return string(p)
}

const (
	containerNameKedaOperator  = "keda-operator"
	containerNameMetricsServer = "keda-metrics-apiserver"
)

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

func EnsureCertInjectionForAPIService(annotation string, annotationValue string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "APIService" {
			apiService := &apiregistrationv1beta1.APIService{}
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

func EnsureCertInjectionForService(serviceName string, annotation string, annotationValue string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
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

func EnsureCertInjectionForDeployment(configMapName string, secretName string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}

			// add Volumes referencing certs in ConfigMap and Secret
			cabundleVolume := corev1.Volume{
				Name: "cabundle",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			}
			certsVolume := corev1.Volume{
				Name: "certs",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: secretName,
					},
				},
			}

			volumes := deploy.Spec.Template.Spec.Volumes
			cabundleVolumeFound := false
			certsVolumeFound := false
			for i := range volumes {
				if volumes[i].Name == "cabundle" {
					volumes[i] = cabundleVolume
					cabundleVolumeFound = true
				}
				if volumes[i].Name == "certs" {
					volumes[i] = certsVolume
					certsVolumeFound = true
				}
			}
			if !cabundleVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, cabundleVolume)
			}
			if !certsVolumeFound {
				deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, certsVolume)
			}

			containers := deploy.Spec.Template.Spec.Containers
			for i := range containers {
				if containers[i].Name == containerNameMetricsServer {

					// mount Volumes referencing certs in ConfigMap and Secret
					cabundleVolumeMount := corev1.VolumeMount{
						Name:      "cabundle",
						MountPath: "/cabundle",
					}

					certsVolumeMount := corev1.VolumeMount{
						Name:      "certs",
						MountPath: "/certs",
					}

					volumeMounts := containers[i].VolumeMounts
					cabundleVolumeMountFound := false
					certsVolumeMountFound := false
					for j := range volumeMounts {
						if volumeMounts[j].Name == "cabundle" {
							volumeMounts[j] = cabundleVolumeMount
							cabundleVolumeMountFound = true
						}
						if volumeMounts[j].Name == "certs" {
							volumeMounts[j] = certsVolumeMount
							certsVolumeMountFound = true
						}
					}
					if !cabundleVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, cabundleVolumeMount)
					}
					if !certsVolumeMountFound {
						containers[i].VolumeMounts = append(containers[i].VolumeMounts, certsVolumeMount)
					}
				}
				break
			}

			if err := scheme.Convert(deploy, u, nil); err != nil {
				return err
			}

		}
		return nil
	}
}

func AddPathsToCerts(values []string, prefixes []Prefix, scheme *runtime.Scheme, logger logr.Logger) []mf.Transformer {
	transforms := []mf.Transformer{}
	for i := range values {
		transforms = append(transforms, replaceContainerArg(values[i], prefixes[i], containerNameMetricsServer, scheme, logger))
	}
	return transforms
}

func ReplaceKedaOperatorLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {

	found := false
	for _, level := range logLevelsKedaOperator {
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
		logger.Info("Ignoring speficied Log level for Keda Operator, it needs to be set to ", strings.Join(logLevelsKedaOperator, ", "), "or an integer value greater than 0")
		return func(u *unstructured.Unstructured) error {
			return nil
		}
	}

	var prefix Prefix = LogLevelKedaOperator
	return replaceContainerArg(logLevel, prefix, containerNameKedaOperator, scheme, logger)
}

func ReplaceKedaOperatorLogTimeFormat(logTimeFormat string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {

	found := false
	for _, format := range logTimeFormatsKedaOperator {
		if logTimeFormat == format {
			found = true
		}
	}

	if !found {
		logger.Info("Ignoring speficied Log format for Keda Operator, it needs to be set to ", strings.Join(logTimeFormatsKedaOperator, ", "))
		return func(u *unstructured.Unstructured) error {
			return nil
		}
	}

	var prefix Prefix = LogTimeFormatKedaOperator
	return replaceContainerArg(logTimeFormat, prefix, containerNameKedaOperator, scheme, logger)
}

func ReplaceMetricsServerLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {

	found := false
	if _, err := strconv.ParseUint(logLevel, 10, 64); err == nil {
		found = true
	}

	if !found {
		logger.Info("Ignoring speficied Log level for Keda Metrics Server, it needs to be set to an integer value greater than 0")
		return func(u *unstructured.Unstructured) error {
			return nil
		}
	}

	var prefix Prefix = LogLevelMetricsServer
	return replaceContainerArg(logLevel, prefix, containerNameMetricsServer, scheme, logger)
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
							argFound = true
							if trimmedArg := strings.TrimPrefix(arg, prefix.String()); trimmedArg != value {
								logger.Info("Replacing", "deployment", container.Name, prefix.String(), value, "previous", trimmedArg)
								containers[i].Args[j] = prefix.String()+value
								changed = true
							}
							break
						}
					}
					if !argFound {
						logger.Info("Adding", "deployment", container.Name, prefix.String(), value)
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
