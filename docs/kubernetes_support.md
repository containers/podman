# Podman Kube Play Support

This document outlines the kube yaml fields that are currently supported by the **podman kube play** command.

Note: **N/A** means that the option cannot be supported in a single-node Podman environment.

## Pod Fields

| Field                                             | Support |
|---------------------------------------------------|---------|
| containers                                        | ✅      |
| initContainers                                    | ✅      |
| imagePullSecrets                                  |         |
| enableServiceLinks                                |         |
| os<nolink>.name                                   |         |
| volumes                                           |         |
| nodeSelector                                      | N/A     |
| nodeName                                          | N/A     |
| affinity.nodeAffinity                             | N/A     |
| affinity.podAffinity                              | N/A     |
| affinity.podAntiAffinity                          | N/A     |
| tolerations.key                                   | N/A     |
| tolerations.operator                              | N/A     |
| tolerations.effect                                | N/A     |
| tolerations.tolerationSeconds                     | N/A     |
| schedulerName                                     | N/A     |
| runtimeClassName                                  |         |
| priorityClassName                                 |         |
| priority                                          |         |
| topologySpreadConstraints.maxSkew                 | N/A     |
| topologySpreadConstraints.topologyKey             | N/A     |
| topologySpreadConstraints.whenUnsatisfiable       | N/A     |
| topologySpreadConstraints.labelSelector           | N/A     |
| topologySpreadConstraints.minDomains              | N/A     |
| restartPolicy                                     | ✅      |
| terminationGracePeriod                            |         |
| activeDeadlineSeconds                             |         |
| readinessGates.conditionType                      |         |
| hostname                                          | ✅      |
| setHostnameAsFQDN                                 |         |
| subdomain                                         |         |
| hostAliases.hostnames                             | ✅      |
| hostAliases.ip                                    | ✅      |
| dnsConfig.nameservers                             | ✅      |
| dnsConfig<nolink>.options.name                    | ✅      |
| dnsConfig.options.value                           | ✅      |
| dnsConfig.searches                                | ✅      |
| dnsPolicy                                         |         |
| hostNetwork                                       | ✅      |
| hostPID                                           |         |
| hostIPC                                           |         |
| shareProcessNamespace                             | ✅      |
| serviceAccountName                                |         |
| automountServiceAccountToken                      |         |
| securityContext.runAsUser                         |         |
| securityContext.runAsNonRoot                      |         |
| securityContext.runAsGroup                        |         |
| securityContext.supplementalGroups                |         |
| securityContext.fsGroup                           |         |
| securityContext.fsGroupChangePolicy               |         |
| securityContext.seccompProfile.type               |         |
| securityContext.seccompProfile.localhostProfile   |         |
| securityContext.seLinuxOptions.level              |         |
| securityContext.seLinuxOptions.role               |         |
| securityContext.seLinuxOptions.type               |         |
| securityContext.seLinuxOptions.user               |         |
| securityContext<nolink>.sysctls.name              |         |
| securityContext.sysctls.value                     |         |
| securityContext.windowsOptions.gmsaCredentialSpec |         |
| securityContext.windowsOptions.hostProcess        |         |
| securityContext.windowsOptions.runAsUserName      |         |

## Container Fields

| Field                                             | Support |
|---------------------------------------------------|---------|
| name                                              | ✅      |
| image                                             | ✅      |
| imagePullPolicy                                   | ✅      |
| command                                           | ✅      |
| args                                              | ✅      |
| workingDir                                        | ✅      |
| ports.containerPort                               | ✅      |
| ports.hostIP                                      | ✅      |
| ports.hostPort                                    | ✅      |
| ports<nolink>.name                                | ✅      |
| ports.protocol                                    | ✅      |
| env<nolink>.name                                  | ✅      |
| env.value                                         | ✅      |
| env.valueFrom.configMapKeyRef.key                 | ✅      |
| env<nolink>.valueFrom.configMapKeyRef.name        | ✅      |
| env.valueFrom.configMapKeyRef.optional            | ✅      |
| env.valueFrom.fieldRef                            | ✅      |
| env.valueFrom.resourceFieldRef                    | ✅      |
| env.valueFrom.secretKeyRef.key                    | ✅      |
| env<nolink>.valueFrom.secretKeyRef.name           | ✅      |
| env.valueFrom.secretKeyRef.optional               | ✅      |
| envFrom<nolink>.configMapRef.name                 | ✅      |
| envFrom.configMapRef.optional                     | ✅      |
| envFrom.prefix                                    |         |
| envFrom<nolink>.secretRef.name                    | ✅      |
| envFrom.secretRef.optional                        | ✅      |
| volumeMounts.mountPath                            | ✅      |
| volumeMounts<nolink>.name                         | ✅      |
| volumeMounts.mountPropagation                     |         |
| volumeMounts.readOnly                             | ✅      |
| volumeMounts.subPath                              |         |
| volumeMounts.subPathExpr                          |         |
| volumeDevices.devicePath                          |         |
| volumeDevices<nolink>.name                        |         |
| resources.limits                                  | ✅      |
| resources.requests                                | ✅      |
| lifecycle.postStart                               |         |
| lifecycle.preStop                                 |         |
| terminationMessagePath                            |         |
| terminationMessagePolicy                          |         |
| livenessProbe                                     | ✅      |
| readinessProbe                                    |         |
| startupProbe                                      |         |
| securityContext.runAsUser                         | ✅      |
| securityContext.runAsNonRoot                      |         |
| securityContext.runAsGroup                        | ✅      |
| securityContext.readOnlyRootFilesystem            | ✅      |
| securityContext.procMount                         |         |
| securityContext.privileged                        | ✅      |
| securityContext.allowPrivilegeEscalation          | ✅      |
| securityContext.capabilities.add                  | ✅      |
| securityContext.capabilities.drop                 | ✅      |
| securityContext.seccompProfile.type               |         |
| securityContext.seccompProfile.localhostProfile   |         |
| securityContext.seLinuxOptions.level              | ✅      |
| securityContext.seLinuxOptions.role               | ✅      |
| securityContext.seLinuxOptions.type               | ✅      |
| securityContext.seLinuxOptions.user               | ✅      |
| securityContext.windowsOptions.gmsaCredentialSpec |         |
| securityContext.windowsOptions.hostProcess        |         |
| securityContext.windowsOptions.runAsUserName      |         |
| stdin                                             |         |
| stdinOnce                                         |         |
| tty                                               |         |

## PersistentVolumeClaim Fields

| Field              | Support |
|--------------------|---------|
| volumeName         |         |
| storageClassName   | ✅      |
| volumeMode         |         |
| accessModes        | ✅      |
| selector           |         |
| resources.limits   |         |
| resources.requests | ✅      |

## ConfigMap Fields

| Field      | Support |
|------------|---------|
| binaryData | ✅      |
| data       | ✅      |
| immutable  |         |

## Deployment Fields

| Field                                 | Support |
|---------------------------------------|---------|
| replicas                              | ✅      |
| selector                              | ✅      |
| template                              | ✅      |
| minReadySeconds                       |         |
| strategy.type                         |         |
| strategy.rollingUpdate.maxSurge       |         |
| strategy.rollingUpdate.maxUnavailable |         |
| revisionHistoryLimit                  |         |
| progressDeadlineSeconds               |         |
| paused                                |         |
