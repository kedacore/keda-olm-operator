<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [KEDA OLM Operator](#keda-olm-operator)
  - [Installation](#installation)
    - [Operator Hub Installation](#operator-hub-installation)
    - [Manual installation](#manual-installation)
  - [The `KedaController` Custom Resource](#the-kedacontroller-custom-resource)
    - [`KedaController` Spec](#kedacontroller-spec)
  - [Uninstallation](#uninstallation)
    - [How to uninstall KEDA Controller](#how-to-uninstall-keda-controller)
    - [How to uninstall KEDA OLM Operator](#how-to-uninstall-keda-olm-operator)
  - [Monitoring](#monitoring)
  - [Development](#development)
    - [Pre-requisites](#pre-requisites)
    - [Operator Framework](#operator-framework)
    - [Running locally](#running-locally)
    - [Building the Operator Image](#building-the-operator-image)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# KEDA OLM Operator

<p style="font-size: 25px" align="center">
<a href="https://github.com/kedacore/keda-olm-operator/actions"><img src="https://github.com/kedacore/keda-olm-operator/workflows/main%20build/badge.svg" alt="main build"></a>
<a href="https://github.com/kedacore/keda-olm-operator/actions"><img src="https://github.com/kedacore/keda-olm-operator/workflows/nightly%20tests/badge.svg" alt="nightly e2e"></a></p>


Operator for deploying [KEDA](https://keda.sh/) (Kubernetes Event-driven Autoscaling) controller on OpenShift or any Kubernetes cluster with
[Operator Lifecycle Manager](https://github.com/operator-framework/operator-lifecycle-manager) framework installed.

## Installation

Please note that you can not run both KEDA v1 and v2 on the same Kubernetes cluster. You need to uninstall KEDA v1 first, in order to install and use KEDA v2.
Don't forget to uninstall KEDA v1 CRDs as well, to ensure that, please run:
```bash
kubectl delete crd scaledobjects.keda.k8s.io
kubectl delete crd triggerauthentications.keda.k8s.io

```


### Operator Hub Installation
1. On Operator Hub Marketplace locate and install `KEDA` operator. Choose a namespace where the operator will be installed. The `keda` namespace is recommended.
2. Create `KedaController` resource in namespace where the operator was installed (e.g. `keda`)

![Operator Hub Installation Demo](images/keda-olm-install.gif)


### Manual installation

The following will install [KEDA](https://github.com/kedacore/keda) and configure it
appropriately for your cluster, please run these commands:

```bash
make deploy                                                                  # deploy KEDA OLM Operator
kubectl apply -n keda -f config/samples/keda_v1alpha1_kedacontroller.yaml    # install KEDA
```

To be clear, the operator will be deployed in the `keda` namespace,
and then it will install KEDA into this namespace.

## The `KedaController` Custom Resource

The installation of KEDA is triggered by the creation of
[a `KedaController` custom resource](config/samples/keda_v1alpha1_kedacontroller.yaml).
Only custom resource named `keda` in the namespace where the operator was
installed (typically, `keda`) will trigger the installation, reconfiguration,
or removal of the KEDA Controller resources.

The operator will behave in this manner whether it is installed with the
`AllNamespaces` or `OwnNamespace` install mode. While the operator more
closely matches the `OwnNamespace` semantics, `AllNamespaces` is a
supported installation mode to allow it to be installed to namespaces with
existing `OperatorGroups` which require that installation mode.

There should be only one KEDA Controller in the cluster.

### `KedaController` Spec
```
apiVersion: keda.sh/v1alpha1
kind: KedaController
metadata:
  name: keda
  namespace: keda
spec:
  ###
  # THERE SHOULD BE ONLY ONE INSTANCE OF THIS RESOURCE PER CLUSTER
  # with Name set to 'keda' created in namespace where the operator is installed (usually 'keda')
  ###

  ## Namespace that should be watched by KEDA,
  # omit or set empty to watch all namespaces (default setting)
  watchNamespace: ""

  ## KEDA Operator related config
  operator:
    ## Logging level for KEDA Operator
    # allowed values: 'debug', 'info', 'error', or an integer value greater than 0, specified as string
    # default value: info
    logLevel: info

    ## Logging format for KEDA Operator
    # allowed values are json and console
    # default value: console
    logEncoder: console

    ## Logging time encoding for KEDA Controller
    # allowed values are 'epoch', 'millis', 'nano', 'iso8601', 'rfc3339' or 'rfc3339nano'
    # default value: rfc3339
    # logTimeEncoding: rfc3339

    ## Arbitrary arguments
    # Define any argument with possibility to override already existing ones.
    # Array of strings (format is either with prefix '--key=value' or just 'value')
    # args: []

    ## Annotations to be added to the KEDA Operator Deployment
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
    # deploymentAnnotations:
    #  annotationKey: annotationValue

    ## Labels to be added to the KEDA Operator Deployment
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # deploymentLabels:
    #  labelKey: labelValue

    ## Annotations to be added to the KEDA Operator Pod
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
    # podAnnotations:
    #  annotationKey: annotationValue

    ## Labels to be added to the KEDA Operator Pod
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # podLabels:
    #  labelKey: labelValue

    ## Node selector for pod scheduling for KEDA Operator
    # https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
    # nodeSelector:
    #  beta.kubernetes.io/os: linux

    ## Tolerations for pod scheduling for KEDA Operator
    # https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
    # tolerations:
    # - key: "key1"
    #   operator: "Equal"
    #   value: "value1"
    #   effect: "NoSchedule"
    # - key: "key1"
    #   operator: "Equal"
    #   value: "value1"
    #   effect: "NoExecute"

    ## Affinity for pod scheduling for KEDA Operator
    # https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/
    # affinity:
    #  podAntiAffinity:
    #    requiredDuringSchedulingIgnoredDuringExecution:
    #     - labelSelector:
    #         matchExpressions:
    #         - key: app
    #           operator: In
    #           values:
    #           - keda-operator
    #           - keda-operator-metrics-apiserver
    #       topologyKey: "kubernetes.io/hostname"

    ## Pod priority for KEDA Operator
    # https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/
    # priorityClassName: high-priority

    ## Manage resource requests & limits for KEDA Operator
    # https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
    # resources:
    #   requests:
    #     cpu: 100m
    #     memory: 100Mi
    #   limits:
    #     cpu: 1000m
    #      memory: 1000Mi

  ## KEDA Metrics Server related config
  metricsServer:
    ## Logging level for Metrics Server
    # allowed values: "0" for info, "4" for debug, or an integer value greater than 0, specified as string
    # default value: "0"
    logLevel: "0"

    ## Arbitrary arguments
    # Define any argument with possibility to override already existing ones.
    # Array of strings (format is either with prefix '--key=value' or just 'value')
    # args: []

    ## Audit Config
    # https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/#audit-policy
    # Define basic arguments for auditing log files. If needed, more complex flags
    # can be set via 'Args' field manually.
    # Non-empty 'policy' field is mandatory to enable logging.
    # If 'logOutputVolumeClaim' is empty the audit log is printed to stdout,
    # otherwise it points to the user defined PersistentVolumeClaim resource name.
    # auditConfig:
    #   logFormat: "json"
    #   logOutputVolumeClaim: "persistentVolumeClaimName"
    #   policy:
    #     rules:
    #     - level: Metadata
    #     omitStages:
    #     - RequestReceived
    #     omitManagedFields: false
    #   lifetime:
    #     maxAge: "2"
    #     maxBackup: "1"
    #     maxSize: "50"

    # --- Audit Config Example 1 ---
    ## Log request metadata but not request or response body to stdout
    # auditConfig:
    #   policy:
    #     rules:
    #     - level: Metadata

    # --- Audit Config Example 2 ---
    ## Log request metadata to PersistentVolumeClaim with max output file size of 50MB
    # auditConfig:
    #   logOutputVolumeClaim: "persistentVolumeClaimName"
    #   policy:
    #     rules:
    #     - level: Metadata
    #   lifetime:
    #     maxSize: "50"

    # --- Audit Config Example 3 ---
    ## Omits all requests in RequestReceived stage, first rule logs pod changes
    ## at RequestResponse level & second rule forbids logging requests to a
    ## configmap called "controller-leader".
    # auditConfig:
    #   policy:
    #     omitStages:
    #      - "RequestReceived"
    #     rules:
    #     - level: RequestResponse
    #       resources:
    #       - group: ""
    #         resources: ["pods"]
    #     - level: None
    #       resources:
    #       - group: ""
    #         resources: ["configmaps"]
    #         resourceNames: ["controller-leader"]

    ## Annotations to be added to the KEDA Metrics Server Deployment
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
    # deploymentAnnotations:
    #  annotationKey: annotationValue

    ## Labels to be added to the KEDA Metrics Server Deployment
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # deploymentLabels:
    #  labelKey: labelValue

    ## Annotations to be added to the KEDA Metrics Server Pod
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
    # podAnnotations:
    #  annotationKey: annotationValue

    ## Labels to be added to the KEDA Metrics Server Pod
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # podLabels:
    #  labelKey: labelValue

    ## Node selector for pod scheduling for Metrics Server
    # https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
    # nodeSelector:
    #  beta.kubernetes.io/os: linux

    ## Tolerations for pod scheduling for KEDA Metrics Server
    # https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
    # tolerations:
    # - key: "key1"
    #   operator: "Equal"
    #   value: "value1"
    #   effect: "NoSchedule"
    # - key: "key1"
    #   operator: "Equal"
    #   value: "value1"
    #   effect: "NoExecute"

    ## Affinity for pod scheduling for KEDA Metrics Server
    # https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/
    # affinity:
    #  podAntiAffinity:
    #    requiredDuringSchedulingIgnoredDuringExecution:
    #     - labelSelector:
    #         matchExpressions:
    #         - key: app
    #           operator: In
    #           values:
    #           - keda-operator
    #           - keda-operator-metrics-apiserver
    #       topologyKey: "kubernetes.io/hostname"

    ## Pod priority for KEDA Metrics Server
    # https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/
    # priorityClassName: high-priority

    ## Manage resource requests & limits for KEDA Metrics Server
    # https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
    # resources:
    #   requests:
    #     cpu: 100m
    #     memory: 100Mi
    #   limits:
    #     cpu: 1000m
    #      memory: 1000Mi

  ## KEDA Admission Webhooks related config
  admissionWebhooks:
    ## Logging level for KEDA Admission Webhooks
    # allowed values: 'debug', 'info', 'error', or an integer value greater than 0, specified as string
    # default value: info
    logLevel: info

    ## Logging format for KEDA Admission Webhooks
    # allowed values are json and console
    # default value: console
    logEncoder: console

    ## Logging time encoding for KEDA Admission Webhooks
    # allowed values are `epoch`, `millis`, `nano`, `iso8601`, `rfc3339` or `rfc3339nano`
    # default value: rfc3339
    # logTimeEncoding: rfc3339

    ## Arbitrary arguments
    # Define any argument with possibility to override already existing ones.
    # Array of strings (format is either with prefix '--key=value' or just 'value')
    # args: []

    ## Annotations to be added to the KEDA Admission Webhooks Deployment
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
    # deploymentAnnotations:
    #  annotationKey: annotationValue

    ## Labels to be added to the KEDA Admission Webhooks Deployment
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # deploymentLabels:
    #  labelKey: labelValue

    ## Annotations to be added to the KEDA Admission Webhooks Pod
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
    # podAnnotations:
    #  annotationKey: annotationValue

    ## Labels to be added to the KEDA Admission Webhooks Pod
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # podLabels:
    #  labelKey: labelValue

    ## Node selector for pod scheduling for KEDA Admission Webhooks
    # https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/
    # nodeSelector:
    #  beta.kubernetes.io/os: linux

    ## Tolerations for pod scheduling for KEDA Admission Webhooks
    # https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
    # tolerations:
    # - key: "key1"
    #   operator: "Equal"
    #   value: "value1"
    #   effect: "NoSchedule"
    # - key: "key1"
    #   operator: "Equal"
    #   value: "value1"
    #   effect: "NoExecute"

    ## Affinity for pod scheduling for KEDA Admission Webhooks
    # https://kubernetes.io/docs/tasks/configure-pod-container/assign-pods-nodes-using-node-affinity/
    # affinity:
    #  podAntiAffinity:
    #    requiredDuringSchedulingIgnoredDuringExecution:
    #     - labelSelector:
    #         matchExpressions:
    #         - key: app
    #           operator: In
    #           values:
    #           - keda-operator
    #           - keda-operator-metrics-apiserver
    #       topologyKey: "kubernetes.io/hostname"

    ## Pod priority for KEDA Admission Webhooks
    # https://kubernetes.io/docs/concepts/configuration/pod-priority-preemption/
    # priorityClassName: high-priority

    ## Manage resource requests & limits for KEDA Admission Webhooks
    # https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
    # resources:
    #   requests:
    #     cpu: 100m
    #     memory: 100Mi
    #   limits:
    #     cpu: 1000m
    #      memory: 1000Mi

  ## KEDA ServiceAccount related config
  serviceAccount:
    ## Annotations to be added to the Service Account
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
    # annotations:
    #  annotationKey: annotationValue

    ## Labels to be added to the ServiceAccount
    # https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
    # labels:
    #  labelKey: labelValue
```


## Uninstallation

### How to uninstall KEDA Controller
Locate installed `KEDA` Operator in `keda` namespace and then remove created `KedaController` resource or simply delete the `KedaController` resource:

```bash
kubectl delete -n keda -f config/samples/keda_v1alpha1_kedacontroller.yaml
```

### How to uninstall KEDA OLM Operator
To remove KEDA OLM Operator from your cluster, on Operator Hub locate and uninstall `KEDA` operator.

In case of manual installation, run these commands:

```bash
make undeploy
```

## Monitoring
This operator contains monitoring configuration to enable Prometheus metrics
collection. ServiceMonitor and PodMonitor instances are created if the CRDs from
the Monitoring API are available in the cluster.

## Development

### Pre-requisites

This project uses the following tools for development.

#### golangci-lint

To install `golangci-lint` locally follow the [official documentation](https://golangci-lint.run/usage/install/#local-installation).

#### pre-commit

To install `pre-commit` locally follow the [official documentation](https://pre-commit.com/#install).
`pre-commit` uses the [.pre-commit-config.yaml](.pre-commit-config.yaml) configuration file located in the root of the
project.

To set up the `git` hook script execute the following command so that the pre-commit steps runs automatically on
each commit.
```bash
pre-commit install
```

### Operator Framework

This operator was created using the
[operator-sdk](https://github.com/operator-framework/operator-sdk/). And uses
[Operator Lifecycle
Manager](https://github.com/operator-framework/operator-lifecycle-manager)
to describe deployment metadata.

### Running locally
It can be convenient to run the operator outside of the cluster to
test changes. The following command will build the operator and use
your current "kube config" to connect to the cluster:

```bash
make install    # install KedaController CRD in the cluster
make run        # run operator locally
```

### Building the Operator Image

To build the operator:

```bash
make build
```
