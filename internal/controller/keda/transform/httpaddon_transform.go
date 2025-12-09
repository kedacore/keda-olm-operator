/*
Copyright 2025 The KEDA Authors

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

package transform

import (
	"strconv"
	"strings"

	"github.com/go-logr/logr"
	mf "github.com/manifestival/manifestival"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	httpAddonNamePrefix                  = "keda-http-add-on"
	containerNameHTTPAddonOperator       = "keda-http-add-on-operator"
	containerNameHTTPAddonScaler         = "keda-http-add-on-external-scaler"
	containerNameHTTPAddonInterceptor    = "keda-http-add-on-interceptor"
	deploymentNameHTTPAddonOperator      = "keda-http-add-on-operator"
	deploymentNameHTTPAddonScaler        = "keda-http-add-on-external-scaler"
	deploymentNameHTTPAddonInterceptor   = "keda-http-add-on-interceptor"
	pdbNameHTTPAddonInterceptor          = "keda-http-add-on-interceptor"
	scaledObjectNameHTTPAddonInterceptor = "keda-http-add-on-interceptor"
	serviceHTTPAddonInterceptorAdmin     = "keda-http-add-on-interceptor-admin"
	serviceHTTPAddonInterceptorProxy     = "keda-http-add-on-interceptor-proxy"
	serviceHTTPAddonExternalScaler       = "keda-http-add-on-external-scaler"
	grpcPortHTTPAddonExternalScaler      = 9090
)

// ReplaceHTTPAddonOperatorImage replaces the operator image
func ReplaceHTTPAddonOperatorImage(image string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerImage(image, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerImage replaces the scaler image
func ReplaceHTTPAddonScalerImage(image string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerImage(image, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorImage replaces the interceptor image
func ReplaceHTTPAddonInterceptorImage(image string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerImage(image, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme)
}

func replaceHTTPAddonContainerImage(image string, containerName string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
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

// ReplaceHTTPAddonOperatorReplicas replaces the operator replicas
func ReplaceHTTPAddonOperatorReplicas(replicas int32, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentReplicas(replicas, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerReplicas replaces the scaler replicas
func ReplaceHTTPAddonScalerReplicas(replicas int32, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentReplicas(replicas, deploymentNameHTTPAddonScaler, scheme)
}

func replaceHTTPAddonDeploymentReplicas(replicas int32, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			deploy.Spec.Replicas = &replicas
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorLogLevel replaces the operator log level
func ReplaceHTTPAddonOperatorLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logLevel, LogLevelArg, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme, logger)
}

// ReplaceHTTPAddonOperatorLogEncoder replaces the operator log encoder
func ReplaceHTTPAddonOperatorLogEncoder(logEncoder string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logEncoder, LogEncoderArg, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme, logger)
}

// ReplaceHTTPAddonOperatorLogTimeEncoding replaces the operator log time encoding
func ReplaceHTTPAddonOperatorLogTimeEncoding(logTimeEncoding string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logTimeEncoding, LogTimeEncodingArg, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme, logger)
}

// ReplaceHTTPAddonScalerLogLevel replaces the scaler log level
func ReplaceHTTPAddonScalerLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logLevel, LogLevelArg, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme, logger)
}

// ReplaceHTTPAddonScalerLogEncoder replaces the scaler log encoder
func ReplaceHTTPAddonScalerLogEncoder(logEncoder string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logEncoder, LogEncoderArg, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme, logger)
}

// ReplaceHTTPAddonScalerLogTimeEncoding replaces the scaler log time encoding
func ReplaceHTTPAddonScalerLogTimeEncoding(logTimeEncoding string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logTimeEncoding, LogTimeEncodingArg, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme, logger)
}

// ReplaceHTTPAddonInterceptorLogLevel replaces the interceptor log level
func ReplaceHTTPAddonInterceptorLogLevel(logLevel string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logLevel, LogLevelArg, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme, logger)
}

// ReplaceHTTPAddonInterceptorLogEncoder replaces the interceptor log encoder
func ReplaceHTTPAddonInterceptorLogEncoder(logEncoder string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logEncoder, LogEncoderArg, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme, logger)
}

// ReplaceHTTPAddonInterceptorLogTimeEncoding replaces the interceptor log time encoding
func ReplaceHTTPAddonInterceptorLogTimeEncoding(logTimeEncoding string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return replaceHTTPAddonLogArg(logTimeEncoding, LogTimeEncodingArg, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme, logger)
}

func replaceHTTPAddonLogArg(value string, prefix Prefix, containerName string, deploymentName string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
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
								logger.Info("Replacing HTTP Add-on arg", "deployment", deploymentName, "container", containerName, "prefix", prefix.String(), "value", value, "previous", trimmedArg)
								containers[i].Args[j] = prefix.String() + value
							}
							break
						}
					}
					if !argFound {
						logger.Info("Adding HTTP Add-on arg", "deployment", deploymentName, "container", containerName, "prefix", prefix.String(), "value", value)
						containers[i].Args = append(containers[i].Args, prefix.String()+value)
					}
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorResources replaces the operator resources
func ReplaceHTTPAddonOperatorResources(resources corev1.ResourceRequirements, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerResources(resources, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerResources replaces the scaler resources
func ReplaceHTTPAddonScalerResources(resources corev1.ResourceRequirements, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerResources(resources, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorResources replaces the interceptor resources
func ReplaceHTTPAddonInterceptorResources(resources corev1.ResourceRequirements, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerResources(resources, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme)
}

func replaceHTTPAddonContainerResources(resources corev1.ResourceRequirements, containerName string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
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

// ReplaceHTTPAddonOperatorNodeSelector replaces the operator node selector
func ReplaceHTTPAddonOperatorNodeSelector(nodeSelector map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentNodeSelector(nodeSelector, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerNodeSelector replaces the scaler node selector
func ReplaceHTTPAddonScalerNodeSelector(nodeSelector map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentNodeSelector(nodeSelector, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorNodeSelector replaces the interceptor node selector
func ReplaceHTTPAddonInterceptorNodeSelector(nodeSelector map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentNodeSelector(nodeSelector, deploymentNameHTTPAddonInterceptor, scheme)
}

func replaceHTTPAddonDeploymentNodeSelector(nodeSelector map[string]string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			// Merge with existing node selector (keep kubernetes.io/os: linux)
			if deploy.Spec.Template.Spec.NodeSelector == nil {
				deploy.Spec.Template.Spec.NodeSelector = make(map[string]string)
			}
			for k, v := range nodeSelector {
				deploy.Spec.Template.Spec.NodeSelector[k] = v
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorTolerations replaces the operator tolerations
func ReplaceHTTPAddonOperatorTolerations(tolerations []corev1.Toleration, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentTolerations(tolerations, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerTolerations replaces the scaler tolerations
func ReplaceHTTPAddonScalerTolerations(tolerations []corev1.Toleration, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentTolerations(tolerations, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorTolerations replaces the interceptor tolerations
func ReplaceHTTPAddonInterceptorTolerations(tolerations []corev1.Toleration, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentTolerations(tolerations, deploymentNameHTTPAddonInterceptor, scheme)
}

func replaceHTTPAddonDeploymentTolerations(tolerations []corev1.Toleration, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
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

// ReplaceHTTPAddonOperatorAffinity replaces the operator affinity
func ReplaceHTTPAddonOperatorAffinity(affinity *corev1.Affinity, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentAffinity(affinity, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerAffinity replaces the scaler affinity
func ReplaceHTTPAddonScalerAffinity(affinity *corev1.Affinity, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentAffinity(affinity, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorAffinity replaces the interceptor affinity
func ReplaceHTTPAddonInterceptorAffinity(affinity *corev1.Affinity, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentAffinity(affinity, deploymentNameHTTPAddonInterceptor, scheme)
}

func replaceHTTPAddonDeploymentAffinity(affinity *corev1.Affinity, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
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

// ReplaceHTTPAddonOperatorPodAnnotations replaces the operator pod annotations
func ReplaceHTTPAddonOperatorPodAnnotations(annotations map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentPodAnnotations(annotations, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerPodAnnotations replaces the scaler pod annotations
func ReplaceHTTPAddonScalerPodAnnotations(annotations map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentPodAnnotations(annotations, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorPodAnnotations replaces the interceptor pod annotations
func ReplaceHTTPAddonInterceptorPodAnnotations(annotations map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonDeploymentPodAnnotations(annotations, deploymentNameHTTPAddonInterceptor, scheme)
}

func replaceHTTPAddonDeploymentPodAnnotations(annotations map[string]string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
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

// ReplaceHTTPAddonInterceptorEnv replaces environment variables for the interceptor
func ReplaceHTTPAddonInterceptorEnv(envName string, envValue string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerEnv(envName, envValue, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme)
}

// ReplaceHTTPAddonScalerEnv replaces environment variables for the scaler
func ReplaceHTTPAddonScalerEnv(envName string, envValue string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerEnv(envName, envValue, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonOperatorEnv replaces environment variables for the operator
func ReplaceHTTPAddonOperatorEnv(envName string, envValue string, scheme *runtime.Scheme) mf.Transformer {
	return replaceHTTPAddonContainerEnv(envName, envValue, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme)
}

func replaceHTTPAddonContainerEnv(envName string, envValue string, containerName string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					envFound := false
					for j, env := range container.Env {
						if env.Name == envName {
							containers[i].Env[j].Value = envValue
							envFound = true
							break
						}
					}
					if !envFound {
						containers[i].Env = append(containers[i].Env, corev1.EnvVar{
							Name:  envName,
							Value: envValue,
						})
					}
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonInterceptorScaledObjectMinReplicas replaces the interceptor ScaledObject minReplicaCount
func ReplaceHTTPAddonInterceptorScaledObjectMinReplicas(minReplicas int32, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ScaledObject" && u.GetName() == scaledObjectNameHTTPAddonInterceptor {
			spec, found, err := unstructured.NestedMap(u.Object, "spec")
			if err != nil || !found {
				return err
			}
			spec["minReplicaCount"] = int64(minReplicas)
			return unstructured.SetNestedMap(u.Object, spec, "spec")
		}
		return nil
	}
}

// ReplaceHTTPAddonInterceptorScaledObjectMaxReplicas replaces the interceptor ScaledObject maxReplicaCount
func ReplaceHTTPAddonInterceptorScaledObjectMaxReplicas(maxReplicas int32, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ScaledObject" && u.GetName() == scaledObjectNameHTTPAddonInterceptor {
			spec, found, err := unstructured.NestedMap(u.Object, "spec")
			if err != nil || !found {
				return err
			}
			spec["maxReplicaCount"] = int64(maxReplicas)
			return unstructured.SetNestedMap(u.Object, spec, "spec")
		}
		return nil
	}
}

// ReplaceHTTPAddonInterceptorScaledObjectPollingInterval replaces the interceptor ScaledObject pollingInterval
func ReplaceHTTPAddonInterceptorScaledObjectPollingInterval(pollingInterval int32, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ScaledObject" && u.GetName() == scaledObjectNameHTTPAddonInterceptor {
			spec, found, err := unstructured.NestedMap(u.Object, "spec")
			if err != nil || !found {
				return err
			}
			spec["pollingInterval"] = int64(pollingInterval)
			return unstructured.SetNestedMap(u.Object, spec, "spec")
		}
		return nil
	}
}

// ReplaceHTTPAddonInterceptorScaledObjectPendingRequests replaces the interceptor ScaledObject pendingRequests metric
func ReplaceHTTPAddonInterceptorScaledObjectPendingRequests(pendingRequests int32, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "ScaledObject" && u.GetName() == scaledObjectNameHTTPAddonInterceptor {
			triggers, found, err := unstructured.NestedSlice(u.Object, "spec", "triggers")
			if err != nil || !found || len(triggers) == 0 {
				return err
			}
			trigger := triggers[0].(map[string]interface{})
			metadata := trigger["metadata"].(map[string]interface{})
			metadata["interceptorTargetPendingRequests"] = strconv.Itoa(int(pendingRequests))
			return unstructured.SetNestedSlice(u.Object, triggers, "spec", "triggers")
		}
		return nil
	}
}

// ReplaceHTTPAddonInterceptorPDBMinAvailable replaces the interceptor PDB minAvailable
func ReplaceHTTPAddonInterceptorPDBMinAvailable(minAvailable int32, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "PodDisruptionBudget" && u.GetName() == pdbNameHTTPAddonInterceptor {
			pdb := &policyv1.PodDisruptionBudget{}
			if err := scheme.Convert(u, pdb, nil); err != nil {
				return err
			}
			pdb.Spec.MinAvailable = &intstr.IntOrString{Type: intstr.Int, IntVal: minAvailable}
			pdb.Spec.MaxUnavailable = nil
			return scheme.Convert(pdb, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonInterceptorPDBMaxUnavailable replaces the interceptor PDB maxUnavailable
func ReplaceHTTPAddonInterceptorPDBMaxUnavailable(maxUnavailable int32, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "PodDisruptionBudget" && u.GetName() == pdbNameHTTPAddonInterceptor {
			pdb := &policyv1.PodDisruptionBudget{}
			if err := scheme.Convert(u, pdb, nil); err != nil {
				return err
			}
			pdb.Spec.MaxUnavailable = &intstr.IntOrString{Type: intstr.Int, IntVal: maxUnavailable}
			pdb.Spec.MinAvailable = nil
			return scheme.Convert(pdb, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonAllNamespaces replaces all occurrences of "keda" namespace with the specified namespace
func ReplaceHTTPAddonAllNamespaces(namespace string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Replace the namespace in the object metadata
		if u.GetNamespace() == "keda" {
			u.SetNamespace(namespace)
		}

		// Replace namespace references in environment variables for deployments
		if u.GetKind() == "Deployment" {
			spec, found, err := unstructured.NestedMap(u.Object, "spec", "template", "spec")
			if err != nil || !found {
				return err
			}

			containers, found, err := unstructured.NestedSlice(spec, "containers")
			if err != nil || !found {
				return err
			}

			for i, c := range containers {
				container := c.(map[string]interface{})
				env, found, _ := unstructured.NestedSlice(container, "env")
				if found {
					for j, e := range env {
						envVar := e.(map[string]interface{})
						if envVar["value"] == "keda" {
							envVar["value"] = namespace
							env[j] = envVar
						}
					}
					container["env"] = env
					containers[i] = container
				}
			}
			spec["containers"] = containers
			return unstructured.SetNestedMap(u.Object, spec, "spec", "template", "spec")
		}

		// Replace namespace in ClusterRoleBinding subjects
		if u.GetKind() == "ClusterRoleBinding" {
			subjects, found, err := unstructured.NestedSlice(u.Object, "subjects")
			if err != nil || !found {
				return err
			}
			for i, s := range subjects {
				subject := s.(map[string]interface{})
				if subject["namespace"] == "keda" {
					subject["namespace"] = namespace
					subjects[i] = subject
				}
			}
			return unstructured.SetNestedSlice(u.Object, subjects, "subjects")
		}

		// Replace namespace in ScaledObject trigger metadata
		if u.GetKind() == "ScaledObject" {
			triggers, found, err := unstructured.NestedSlice(u.Object, "spec", "triggers")
			if err != nil || !found {
				return err
			}
			for i, t := range triggers {
				trigger := t.(map[string]interface{})
				metadata, found, _ := unstructured.NestedStringMap(trigger, "metadata")
				if found {
					if addr, ok := metadata["scalerAddress"]; ok {
						metadata["scalerAddress"] = strings.Replace(addr, ".keda:", "."+namespace+":", 1)
						// Convert map[string]string to map[string]interface{} for SetNestedSlice compatibility
						metadataInterface := make(map[string]interface{})
						for k, v := range metadata {
							metadataInterface[k] = v
						}
						trigger["metadata"] = metadataInterface
						triggers[i] = trigger
					}
				}
			}
			return unstructured.SetNestedSlice(u.Object, triggers, "spec", "triggers")
		}

		return nil
	}
}

// RemoveSeccompProfileFromHTTPAddonInterceptor removes SeccompProfile from the interceptor deployment
func RemoveSeccompProfileFromHTTPAddonInterceptor(scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return RemoveSeccompProfile(containerNameHTTPAddonInterceptor, scheme, logger)
}

// RemoveSeccompProfileFromHTTPAddonOperator removes SeccompProfile from the operator deployment
func RemoveSeccompProfileFromHTTPAddonOperator(scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return RemoveSeccompProfile(containerNameHTTPAddonOperator, scheme, logger)
}

// RemoveSeccompProfileFromHTTPAddonScaler removes SeccompProfile from the scaler deployment
func RemoveSeccompProfileFromHTTPAddonScaler(scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return RemoveSeccompProfile(containerNameHTTPAddonScaler, scheme, logger)
}

// AddHTTPAddonAdditionalLabels adds additional labels to all HTTP Add-on resources
func AddHTTPAddonAdditionalLabels(labels map[string]string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Check if this is an HTTP Add-on resource by checking name prefix
		if !strings.HasPrefix(u.GetName(), httpAddonNamePrefix) {
			return nil
		}
		existingLabels := u.GetLabels()
		if existingLabels == nil {
			existingLabels = make(map[string]string)
		}
		for k, v := range labels {
			existingLabels[k] = v
		}
		u.SetLabels(existingLabels)
		return nil
	}
}

// ReplaceHTTPAddonPodSecurityContext replaces pod security context for a deployment
func ReplaceHTTPAddonPodSecurityContext(podSecurityContext *corev1.PodSecurityContext, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			deploy.Spec.Template.Spec.SecurityContext = podSecurityContext
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorPodSecurityContext replaces pod security context for the operator deployment
func ReplaceHTTPAddonOperatorPodSecurityContext(podSecurityContext *corev1.PodSecurityContext, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonPodSecurityContext(podSecurityContext, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerPodSecurityContext replaces pod security context for the scaler deployment
func ReplaceHTTPAddonScalerPodSecurityContext(podSecurityContext *corev1.PodSecurityContext, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonPodSecurityContext(podSecurityContext, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorPodSecurityContext replaces pod security context for the interceptor deployment
func ReplaceHTTPAddonInterceptorPodSecurityContext(podSecurityContext *corev1.PodSecurityContext, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonPodSecurityContext(podSecurityContext, deploymentNameHTTPAddonInterceptor, scheme)
}

// ReplaceHTTPAddonContainerSecurityContext replaces container security context for a container
func ReplaceHTTPAddonContainerSecurityContext(securityContext *corev1.SecurityContext, containerName string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					containers[i].SecurityContext = securityContext
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorSecurityContext replaces container security context for the operator
func ReplaceHTTPAddonOperatorSecurityContext(securityContext *corev1.SecurityContext, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonContainerSecurityContext(securityContext, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerSecurityContext replaces container security context for the scaler
func ReplaceHTTPAddonScalerSecurityContext(securityContext *corev1.SecurityContext, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonContainerSecurityContext(securityContext, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorSecurityContext replaces container security context for the interceptor
func ReplaceHTTPAddonInterceptorSecurityContext(securityContext *corev1.SecurityContext, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonContainerSecurityContext(securityContext, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme)
}

// ReplaceHTTPAddonImagePullSecrets replaces image pull secrets for a deployment
func ReplaceHTTPAddonImagePullSecrets(imagePullSecrets []corev1.LocalObjectReference, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			deploy.Spec.Template.Spec.ImagePullSecrets = imagePullSecrets
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorImagePullSecrets replaces image pull secrets for the operator deployment
func ReplaceHTTPAddonOperatorImagePullSecrets(imagePullSecrets []corev1.LocalObjectReference, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonImagePullSecrets(imagePullSecrets, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerImagePullSecrets replaces image pull secrets for the scaler deployment
func ReplaceHTTPAddonScalerImagePullSecrets(imagePullSecrets []corev1.LocalObjectReference, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonImagePullSecrets(imagePullSecrets, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorImagePullSecrets replaces image pull secrets for the interceptor deployment
func ReplaceHTTPAddonInterceptorImagePullSecrets(imagePullSecrets []corev1.LocalObjectReference, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonImagePullSecrets(imagePullSecrets, deploymentNameHTTPAddonInterceptor, scheme)
}

// ReplaceHTTPAddonContainerPullPolicy replaces image pull policy for a container
func ReplaceHTTPAddonContainerPullPolicy(pullPolicy corev1.PullPolicy, containerName string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					containers[i].ImagePullPolicy = pullPolicy
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorPullPolicy replaces image pull policy for the operator
func ReplaceHTTPAddonOperatorPullPolicy(pullPolicy corev1.PullPolicy, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonContainerPullPolicy(pullPolicy, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerPullPolicy replaces image pull policy for the scaler
func ReplaceHTTPAddonScalerPullPolicy(pullPolicy corev1.PullPolicy, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonContainerPullPolicy(pullPolicy, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorPullPolicy replaces image pull policy for the interceptor
func ReplaceHTTPAddonInterceptorPullPolicy(pullPolicy corev1.PullPolicy, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonContainerPullPolicy(pullPolicy, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme)
}

// ReplaceHTTPAddonTopologySpreadConstraints replaces topology spread constraints for a deployment
func ReplaceHTTPAddonTopologySpreadConstraints(constraints []corev1.TopologySpreadConstraint, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			deploy.Spec.Template.Spec.TopologySpreadConstraints = constraints
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// ReplaceHTTPAddonOperatorTopologySpreadConstraints replaces topology spread constraints for the operator deployment
func ReplaceHTTPAddonOperatorTopologySpreadConstraints(constraints []corev1.TopologySpreadConstraint, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonTopologySpreadConstraints(constraints, deploymentNameHTTPAddonOperator, scheme)
}

// ReplaceHTTPAddonScalerTopologySpreadConstraints replaces topology spread constraints for the scaler deployment
func ReplaceHTTPAddonScalerTopologySpreadConstraints(constraints []corev1.TopologySpreadConstraint, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonTopologySpreadConstraints(constraints, deploymentNameHTTPAddonScaler, scheme)
}

// ReplaceHTTPAddonInterceptorTopologySpreadConstraints replaces topology spread constraints for the interceptor deployment
func ReplaceHTTPAddonInterceptorTopologySpreadConstraints(constraints []corev1.TopologySpreadConstraint, scheme *runtime.Scheme) mf.Transformer {
	return ReplaceHTTPAddonTopologySpreadConstraints(constraints, deploymentNameHTTPAddonInterceptor, scheme)
}

// AddHTTPAddonExtraEnvs adds extra environment variables to a container
func AddHTTPAddonExtraEnvs(extraEnvs map[string]string, containerName string, deploymentName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentName {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerName {
					for name, value := range extraEnvs {
						envFound := false
						for j, env := range container.Env {
							if env.Name == name {
								containers[i].Env[j].Value = value
								envFound = true
								break
							}
						}
						if !envFound {
							containers[i].Env = append(containers[i].Env, corev1.EnvVar{
								Name:  name,
								Value: value,
							})
						}
					}
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// AddHTTPAddonOperatorExtraEnvs adds extra environment variables to the operator
func AddHTTPAddonOperatorExtraEnvs(extraEnvs map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return AddHTTPAddonExtraEnvs(extraEnvs, containerNameHTTPAddonOperator, deploymentNameHTTPAddonOperator, scheme)
}

// AddHTTPAddonScalerExtraEnvs adds extra environment variables to the scaler
func AddHTTPAddonScalerExtraEnvs(extraEnvs map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return AddHTTPAddonExtraEnvs(extraEnvs, containerNameHTTPAddonScaler, deploymentNameHTTPAddonScaler, scheme)
}

// AddHTTPAddonInterceptorExtraEnvs adds extra environment variables to the interceptor
func AddHTTPAddonInterceptorExtraEnvs(extraEnvs map[string]string, scheme *runtime.Scheme) mf.Transformer {
	return AddHTTPAddonExtraEnvs(extraEnvs, containerNameHTTPAddonInterceptor, deploymentNameHTTPAddonInterceptor, scheme)
}

// DeleteHTTPAddonPDB deletes the PodDisruptionBudget by setting its name to empty
func DeleteHTTPAddonPDB() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "PodDisruptionBudget" && u.GetName() == pdbNameHTTPAddonInterceptor {
			u.SetName("")
		}
		return nil
	}
}

// AddHTTPAddonAggregateToDefaultRoles adds aggregate labels to ClusterRoles to enable aggregation to default edit and view roles
func AddHTTPAddonAggregateToDefaultRoles() mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "ClusterRole" {
			return nil
		}
		// Only add aggregate labels to HTTP Add-on ClusterRoles
		if !strings.HasPrefix(u.GetName(), httpAddonNamePrefix) {
			return nil
		}
		labels := u.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		// Add aggregate labels for edit and view roles
		labels["rbac.authorization.k8s.io/aggregate-to-admin"] = "true"
		labels["rbac.authorization.k8s.io/aggregate-to-edit"] = "true"
		labels["rbac.authorization.k8s.io/aggregate-to-view"] = "true"
		u.SetLabels(labels)
		return nil
	}
}

// AddHTTPAddonInterceptorTLSSecretVolume adds a TLS secret volume mount to the interceptor deployment
func AddHTTPAddonInterceptorTLSSecretVolume(secretName string, certPath string, keyPath string, scheme *runtime.Scheme, logger logr.Logger) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != "Deployment" || u.GetName() != deploymentNameHTTPAddonInterceptor {
			return nil
		}

		deploy := &appsv1.Deployment{}
		if err := scheme.Convert(u, deploy, nil); err != nil {
			return err
		}

		// Determine mount path from certPath (use directory part)
		mountPath := "/certs"
		certDir := ""
		keyDir := ""

		if certPath != "" {
			lastSlash := strings.LastIndex(certPath, "/")
			if lastSlash > 0 {
				certDir = certPath[:lastSlash]
				mountPath = certDir
			}
		}

		if keyPath != "" {
			lastSlash := strings.LastIndex(keyPath, "/")
			if lastSlash > 0 {
				keyDir = keyPath[:lastSlash]
			}
		}

		// Validate that cert and key are in the same directory
		if certDir != "" && keyDir != "" && certDir != keyDir {
			logger.Info("Warning: TLS certPath and keyPath are in different directories; the secret will be mounted at certPath directory",
				"certPath", certPath, "keyPath", keyPath, "mountPath", mountPath)
		}

		// Add the volume
		volumeName := "tls-certs"
		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: secretName,
				},
			},
		}

		// Check if volume already exists
		volumeExists := false
		for i, v := range deploy.Spec.Template.Spec.Volumes {
			if v.Name == volumeName {
				deploy.Spec.Template.Spec.Volumes[i] = volume
				volumeExists = true
				break
			}
		}
		if !volumeExists {
			deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, volume)
		}

		// Add the volume mount to the interceptor container
		volumeMount := corev1.VolumeMount{
			Name:      volumeName,
			MountPath: mountPath,
			ReadOnly:  true,
		}

		containers := deploy.Spec.Template.Spec.Containers
		for i, container := range containers {
			if container.Name == containerNameHTTPAddonInterceptor {
				mountExists := false
				for j, vm := range container.VolumeMounts {
					if vm.Name == volumeName {
						containers[i].VolumeMounts[j] = volumeMount
						mountExists = true
						break
					}
				}
				if !mountExists {
					containers[i].VolumeMounts = append(containers[i].VolumeMounts, volumeMount)
				}
				break
			}
		}

		return scheme.Convert(deploy, u, nil)
	}
}

// ReplaceHTTPAddonServiceName renames a Service resource
func ReplaceHTTPAddonServiceName(oldName string, newName string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Service" && u.GetName() == oldName {
			u.SetName(newName)
		}
		return nil
	}
}

// ReplaceHTTPAddonInterceptorAdminServiceName renames the interceptor admin service
func ReplaceHTTPAddonInterceptorAdminServiceName(newName string) mf.Transformer {
	return ReplaceHTTPAddonServiceName(serviceHTTPAddonInterceptorAdmin, newName)
}

// ReplaceHTTPAddonInterceptorProxyServiceName renames the interceptor proxy service
func ReplaceHTTPAddonInterceptorProxyServiceName(newName string) mf.Transformer {
	return ReplaceHTTPAddonServiceName(serviceHTTPAddonInterceptorProxy, newName)
}

// ReplaceHTTPAddonExternalScalerServiceName renames the external scaler service and updates references
func ReplaceHTTPAddonExternalScalerServiceName(newName string, namespace string, port int32, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		// Rename the Service itself
		if u.GetKind() == "Service" && u.GetName() == serviceHTTPAddonExternalScaler {
			u.SetName(newName)
			return nil
		}

		// Update the operator deployment env var that references the scaler service
		if u.GetKind() == "Deployment" && u.GetName() == deploymentNameHTTPAddonOperator {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerNameHTTPAddonOperator {
					for j, env := range container.Env {
						if env.Name == "KEDAHTTP_OPERATOR_EXTERNAL_SCALER_SERVICE" {
							containers[i].Env[j].Value = newName
							break
						}
					}
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}

		// Update the ScaledObject trigger metadata that references the scaler service
		if u.GetKind() == "ScaledObject" && u.GetName() == scaledObjectNameHTTPAddonInterceptor {
			triggers, found, err := unstructured.NestedSlice(u.Object, "spec", "triggers")
			if err != nil || !found || len(triggers) == 0 {
				return err
			}
			for i, t := range triggers {
				trigger := t.(map[string]interface{})
				metadata, found, _ := unstructured.NestedStringMap(trigger, "metadata")
				if found {
					if _, ok := metadata["scalerAddress"]; ok {
						// Update the scaler address with new service name
						if port == 0 {
							port = grpcPortHTTPAddonExternalScaler
						}

						metadata["scalerAddress"] = newName + "." + namespace + ":" + strconv.Itoa(int(port))
						metadataInterface := make(map[string]interface{})
						for k, v := range metadata {
							metadataInterface[k] = v
						}
						trigger["metadata"] = metadataInterface
						triggers[i] = trigger
					}
				}
			}
			return unstructured.SetNestedSlice(u.Object, triggers, "spec", "triggers")
		}

		return nil
	}
}

// ReplaceHTTPAddonInterceptorAdminServiceNameInScaler updates the scaler deployment env var that references the admin service
func ReplaceHTTPAddonInterceptorAdminServiceNameInScaler(newName string, scheme *runtime.Scheme) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() == "Deployment" && u.GetName() == deploymentNameHTTPAddonScaler {
			deploy := &appsv1.Deployment{}
			if err := scheme.Convert(u, deploy, nil); err != nil {
				return err
			}
			containers := deploy.Spec.Template.Spec.Containers
			for i, container := range containers {
				if container.Name == containerNameHTTPAddonScaler {
					for j, env := range container.Env {
						if env.Name == "KEDA_HTTP_SCALER_TARGET_ADMIN_SERVICE" {
							containers[i].Env[j].Value = newName
							break
						}
					}
					break
				}
			}
			return scheme.Convert(deploy, u, nil)
		}
		return nil
	}
}

// InjectHTTPAddonOwner creates a Transformer which adds an OwnerReference
// pointing to `owner`, but only if the object is in the same namespace as `owner`
func InjectHTTPAddonOwner(owner mf.Owner) mf.Transformer {
	return InjectOwner(owner)
}
