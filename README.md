<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [KEDA OLM Operator](#keda-olm-operator)
  - [Installation](#installation)
    - [Operator Hub Installation](#operator-hub-installation)
    - [Manual installation](#manual-installation)
  - [The `KedaController` Custom Resource](#the-kedacontroller-custom-resource)
    - [`KedaController` Spec](#kedacontroller-spec)
    - [Configuring KEDA Behavior via Environment Variables](#configuring-keda-behavior-via-environment-variables)
  - [GCP Workload Identity Federation (OpenShift)](#gcp-workload-identity-federation-openshift)
    - [Prerequisites](#prerequisites)
    - [Installation](#installation-1)
    - [What the operator configures](#what-the-operator-configures)
    - [Using it in a ScaledObject](#using-it-in-a-scaledobject)
  - [HTTP Add-on](#http-add-on)
    - [Enabling the HTTP Add-on](#enabling-the-http-add-on)
    - [Image Configuration](#image-configuration)
    - [Component Configuration](#component-configuration)
    - [Configuring HTTP Add-on Behavior via Environment Variables](#configuring-http-add-on-behavior-via-environment-variables)
    - [HTTP Add-on Status](#http-add-on-status)
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

    ## CA Certificate ConfigMap Names
    # ConfigMaps containing PEM-encoded trusted certificate authorities (CAs).
    # The files from the ConfigMaps will be loaded by the KEDA operator during
    # start-up and will be used by scalers to authenticate TLS-enabled metrics
    # data sources.
    # default value: []
    # caConfigMaps: []

    ## Arbitrary arguments
    # Define any argument with possibility to override already existing ones.
    # Array of strings (format is either with prefix '--key=value' or just 'value')
    # args: []

    ## Environment variables
    # Set any environment variable on the KEDA Operator container, overriding a
    # variable of the same name that the operator sets itself.
    # env:
    # - name: KEDA_HTTP_DEFAULT_TIMEOUT
    #   value: "10000"

    ## Egress Network Policy Allow All
    # By default, the operator will be permitted to reach any network endpoint to facilitate scalers and
    # allow them to reach their data sources
    # If set to "false", the allow all policy will be removed, and a cluster admin can create tailored policies
    # to match their network security requirements
    # networkEgressAllowAll: "true"

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

    ## Environment variables
    # Set any environment variable on the KEDA Metrics Server container, overriding a
    # variable of the same name that the operator sets itself.
    # env:
    # - name: KEDA_HTTP_DEFAULT_TIMEOUT
    #   value: "10000"

    ## Egress Network Policy Allow All
    # By default, the metrics server will be permitted to reach any network endpoint to facilitate scalers and
    # allow them to reach their data sources
    # If set to "false", the allow all policy will be removed, and a cluster admin can create tailored policies
    # to match their network security requirements
    # networkEgressAllowAll: "true"

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

    ## Environment variables
    # Set any environment variable on the KEDA Admission Webhooks container, overriding a
    # variable of the same name that the operator sets itself.
    # env:
    # - name: KEDA_HTTP_DEFAULT_TIMEOUT
    #   value: "10000"

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

  ## KEDA HTTP Add-on related config
  httpAddon:
    ## Enable the HTTP Add-on installation
    # When false (default), no HTTP Add-on components are deployed.
    # When true, the operator, interceptor, and scaler are installed.
    # default: false
    enabled: false

    ## Global version tag for HTTP Add-on images
    # Used as image tag for all components unless overridden per component.
    # version: "0.14.0"

    ## HTTP Add-on Operator configuration
    # operator:
    #   logLevel: info
    #   logEncoder: console
    #   image:
    #     name: ghcr.io/kedacore/http-add-on-operator
    #     tag: ""
    #   replicas: 1
    #   env: []
    #   nodeSelector: {}
    #   tolerations: []
    #   affinity: {}
    #   resources: {}

    ## HTTP Add-on Interceptor configuration
    # interceptor:
    #   logLevel: info
    #   logEncoder: console
    #   image:
    #     name: ghcr.io/kedacore/http-add-on-interceptor
    #     tag: ""
    #   replicas: 1
    #   env: []
    #   nodeSelector: {}
    #   tolerations: []
    #   affinity: {}
    #   resources: {}

    ## HTTP Add-on Scaler configuration
    # scaler:
    #   logLevel: info
    #   logEncoder: console
    #   image:
    #     name: ghcr.io/kedacore/http-add-on-scaler
    #     tag: ""
    #   replicas: 1
    #   env: []
    #   nodeSelector: {}
    #   tolerations: []
    #   affinity: {}
    #   resources: {}
```

### Configuring KEDA Behavior via Environment Variables

KEDA settings that are not exposed as dedicated CRD fields can be configured by
passing environment variables directly to the relevant component. Every component
accepts an `env` list of standard Kubernetes
[`EnvVar`](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/)
entries, so both literal values and `valueFrom` references work:

```yaml
spec:
  operator:
    env:
      - name: KEDA_HTTP_DEFAULT_TIMEOUT
        value: "10000"
      - name: HTTPS_PROXY
        valueFrom:
          secretKeyRef:
            name: egress-proxy
            key: url
  metricsServer:
    env:
      - name: KEDA_HTTP_DEFAULT_TIMEOUT
        value: "10000"
  admissionWebhooks:
    env:
      - name: KEDA_HTTP_DEFAULT_TIMEOUT
        value: "10000"
```

A variable is matched by name: one that already exists on the container is
overwritten in place, and anything else is appended. These values are applied
after the ones the operator sets itself, so a user-supplied variable always wins.
Take care with variables the operator manages on your behalf — overriding
`WATCH_NAMESPACE` shadows `spec.watchNamespace`, and on OpenShift overriding
`KEDA_SERVICE_MIN_TLS_VERSION` or `KEDA_SERVICE_TLS_CIPHER_LIST` bypasses the TLS
settings derived from the cluster TLS profile.

Refer to the [KEDA documentation](https://keda.sh/docs/latest/operate/cluster/)
for the full list of supported environment variables.

## GCP Workload Identity Federation (OpenShift)

On an OpenShift cluster that runs on Google Cloud with short-term credentials
(`credentialsMode: Manual` with Workload Identity Federation set up by `ccoctl`),
the operator can give KEDA access to Google Cloud without storing a service
account key anywhere in the cluster. It builds a GCP `external_account`
credential configuration, stores it in the `keda-gcp-credentials` Secret and
wires it into the `keda-operator` Deployment together with a projected Kubernetes
service account token. Scalers that use `podIdentity.provider: gcp` then obtain
short-lived access tokens by impersonating the configured Google service account.

### Prerequisites

1. A Google service account with the roles the scalers need (for example
   `roles/monitoring.viewer` for the Stackdriver and Google Managed Prometheus
   scalers).
2. Permission for the `keda-operator` Kubernetes service account to impersonate
   it. The projected token identifies the workload by its subject, so grant
   `roles/iam.workloadIdentityUser` on the Google service account to

   ```text
   principal://iam.googleapis.com/projects/<project_number>/locations/global/workloadIdentityPools/<pool_id>/subject/system:serviceaccount:<keda_namespace>:keda-operator
   ```

   where `<keda_namespace>` is the namespace the operator is installed in
   (`keda` by default).

### Installation

The parameters are passed to the operator as environment variables of the
`Subscription`. When installing from the OpenShift web console on a Workload
Identity enabled cluster, the install form asks for them; on the command line,
set them in `spec.config.env`:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: keda
  namespace: keda
spec:
  channel: stable
  name: keda
  source: operatorhubio-catalog
  sourceNamespace: olm
  config:
    env:
      - name: AUDIENCE
        value: //iam.googleapis.com/projects/<project_number>/locations/global/workloadIdentityPools/<pool_id>/providers/<provider_id>
      - name: SERVICE_ACCOUNT_EMAIL
        value: <name>@<project_id>.iam.gserviceaccount.com
```

| Variable | Set by | Description |
|---|---|---|
| `SERVICE_ACCOUNT_EMAIL` | console and CLI | Google service account that KEDA impersonates. Required. |
| `AUDIENCE` | CLI | Workload identity provider resource name, `//iam.googleapis.com/projects/<project_number>/locations/global/workloadIdentityPools/<pool_id>/providers/<provider_id>`. Required unless the three variables below are set. |
| `PROJECT_NUMBER`, `POOL_ID`, `PROVIDER_ID` | console | Components of the audience. The console install form collects them; on the CLI you can set them instead of `AUDIENCE`. |
| `CLOUDSDK_CORE_PROJECT` | optional | Google project that scalers default to when the trigger doesn't name one. Defaults to the project the cluster runs in. |
| `SUBJECT_TOKEN_AUDIENCE` | optional | Audience of the projected Kubernetes service account token. Must be one of the allowed audiences of the workload identity provider. Defaults to `openshift`, which is what `ccoctl` configures. |

Workload Identity is enabled as soon as any of these variables other than
`CLOUDSDK_CORE_PROJECT` is set. An incomplete or inconsistent configuration is reported in the `KedaController`
status (`Installation Failed` with the reason in `status.reason`) and KEDA is not
installed until it is fixed. Removing the variables from the `Subscription` removes
the Secret and the Deployment wiring again.

### What the operator configures

- The `keda-gcp-credentials` Secret in the operator namespace with a
  `service_account.json` key holding the `external_account` configuration. The
  Secret is owned by the `KedaController` and is deleted with it. Its layout is
  the same as the one the OpenShift Cloud Credential Operator produces.
- On the `keda-operator` Deployment: a `bound-sa-token` projected volume mounted
  at `/var/run/secrets/openshift/serviceaccount`, the Secret mounted at
  `/var/run/secrets/gcp`, `GOOGLE_APPLICATION_CREDENTIALS` pointing at the
  credential file and `CLOUDSDK_CORE_PROJECT` with the resolved project.
- A `kedacontroller.keda.sh/gcp-credentials-hash` pod annotation, so that a
  changed configuration restarts the `keda-operator` pods.

`spec.operator.env` on the `KedaController` is applied after this wiring, so it
can still override `CLOUDSDK_CORE_PROJECT` if needed.

### Using it in a ScaledObject

Reference a `TriggerAuthentication` with GCP pod identity from the trigger:

```yaml
apiVersion: keda.sh/v1alpha1
kind: TriggerAuthentication
metadata:
  name: gcp-workload-identity
spec:
  podIdentity:
    provider: gcp
---
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: pubsub-consumer
spec:
  scaleTargetRef:
    name: pubsub-consumer
  triggers:
    - type: gcp-pubsub
      authenticationRef:
        name: gcp-workload-identity
      metadata:
        subscriptionName: my-subscription
        mode: SubscriptionSize
        value: "5"
```

## HTTP Add-on

The [KEDA HTTP Add-on](https://github.com/kedacore/http-add-on) enables HTTP-based
autoscaling, allowing Kubernetes deployments to scale (including to and from zero)
based on incoming HTTP traffic. The operator manages the full lifecycle of the
HTTP Add-on components: the **operator**, **interceptor**, and **scaler**.

The HTTP Add-on CRDs (`InterceptorRoute`, `HTTPScaledObject`) are always installed
with the KEDA OLM Operator, regardless of whether the add-on itself is enabled.
This ensures that user-created resources are never accidentally deleted when the
add-on is disabled.

### Enabling the HTTP Add-on

To install the HTTP Add-on, set `httpAddon.enabled: true` and specify a `version`
in the `KedaController` resource:

```yaml
apiVersion: keda.sh/v1alpha1
kind: KedaController
metadata:
  name: keda
  namespace: keda
spec:
  httpAddon:
    enabled: true
    version: "0.14.0"
```

Setting `enabled: false` (the default) will remove all HTTP Add-on deployments
while preserving the CRDs and any user-created `InterceptorRoute` resources.

### Image Configuration

A global `version` field sets the default image tag for all HTTP Add-on components.
Each component can override the tag individually via its `image.tag` field.
If a component's `tag` is empty or not set, the global `version` is used.

```yaml
spec:
  httpAddon:
    enabled: true
    version: "0.14.0"
    operator:
      image:
        name: ghcr.io/kedacore/http-add-on-operator
        tag: "0.14.1"
    interceptor:
      image:
        name: ghcr.io/kedacore/http-add-on-interceptor
        # tag is empty, uses global version 0.14.0
    scaler:
      image:
        name: ghcr.io/kedacore/http-add-on-scaler
```

The example above produces the following images:
- **operator**: `ghcr.io/kedacore/http-add-on-operator:0.14.1` (component override)
- **interceptor**: `ghcr.io/kedacore/http-add-on-interceptor:0.14.0` (global version)
- **scaler**: `ghcr.io/kedacore/http-add-on-scaler:0.14.0` (global version)

### Component Configuration

Each component (operator, interceptor, scaler) supports the following standard
Kubernetes deployment fields:

| Field | Description |
|---|---|
| `logLevel` | Logging level (`debug`, `info`, `error`). Default: `info` |
| `logEncoder` | Logging format (`json`, `console`). Default: `console` |
| `logTimeEncoding` | Time encoding (`epoch`, `millis`, `nano`, `iso8601`, `rfc3339`, `rfc3339nano`). Default: `rfc3339` |
| `image.name` | Container image repository |
| `image.tag` | Image tag override (falls back to global `version`) |
| `replicas` | Number of deployment replicas |
| `env` | Extra environment variables |
| `deploymentAnnotations` | Annotations on the Deployment |
| `deploymentLabels` | Labels on the Deployment |
| `podAnnotations` | Annotations on the Pod |
| `podLabels` | Labels on the Pod |
| `nodeSelector` | Node selector for pod scheduling |
| `tolerations` | Tolerations for pod scheduling |
| `affinity` | Affinity rules for pod scheduling |
| `priorityClassName` | Pod priority class |
| `resources` | Resource requests and limits |

### Configuring HTTP Add-on Behavior via Environment Variables

HTTP Add-on specific settings (such as interceptor timeouts, connection pool sizes,
or scaler parameters) are not exposed as dedicated CRD fields. Instead, configure
them by passing environment variables directly to the relevant component:

```yaml
spec:
  httpAddon:
    enabled: true
    version: "0.14.0"
    interceptor:
      env:
        - name: KEDA_HTTP_CONNECT_TIMEOUT
          value: "500ms"
        - name: KEDA_HTTP_KEEP_ALIVE
          value: "1s"
        - name: KEDA_RESPONSE_HEADER_TIMEOUT
          value: "500ms"
    scaler:
      env:
        - name: KEDA_HTTP_SCALER_TARGET_ADMIN_PORT
          value: "9091"
```

Refer to the [KEDA HTTP Add-on documentation](https://keda.sh/http-add-on)
for the full list of supported environment variables for each component.

### HTTP Add-on Status

When the HTTP Add-on is enabled, the `KedaController` status includes information
about the add-on installation:

```yaml
status:
  httpAddon:
    phase: "Installation Succeeded"
    reason: "HTTP Add-on v0.14.0 is installed in namespace 'keda'"
    version: "0.14.0"
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

To install `golangci-lint` locally follow the [official documentation](https://golangci-lint.run/docs/welcome/install/local/).

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
