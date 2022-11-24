# Workflow Operator

## Overview

This repository introduces the [Workflow Custom Resource Definition (CRD)](config/crd/bases/st4sd.ibm.com_workflows.yaml)
 of the Simulation Toolkit for Scientific Discovery (ST4SD). The workflow operator manages the lifecycle of [Workflow objects](config/samples/st4sd_v1alpha1_workflow.yaml)


## Default values of Workflow Schema

The workflow operator will use the contents of the `consumable-computing-config` ConfigMap (the ConfigMap that the [st4sd-runtime-service](https://github.ibm.com/st4sd/st4sd-runtime-service) uses) to fill in default values for information missing from the workflow YAML. This ConfigMap is also used by the Consumable Computing REST-API to generate Workflow YAML definitions.

The workflow operator will parse the `config.json` data field of the ConfigMap and treat it as a JSON dictionary. It will then use the dictionary key/value pairs to fill in any missing fields of the Workflow YAML like so:

- `spec.image` using the `image` JSON key
- `spec.package.gitsecret` using the `gitsecret` and `gitsecret-oauth` JSON keys (depending on whether `spec.package.url` begins with `git@` or `https://` respectively)
- `spec.imagePullSecrets` using the `imagePullSecrets` JSON key
- `spec.workingVolume` using the `workingVolume` JSON key as the name of a PersistentVolumeClaim
- `spec.s3FetchFilesImage` using the `s3-fetch-files-image` JSON key

## Kubernetes Workflow schema

The full definition of the workflow schema is under [`config/crd/bases/st4sd.ibm.com_workflows.yaml`](config/crd/bases/st4sd.ibm.com_workflows.yaml).

__*NOTE*: This object used to have apiVersion: hpsys.ie.ibm.com__

```yaml
apiVersion: st4sd.ibm.com/v1alpha1
kind: Workflow
metadata:
  name: example-workflow
  labels:
    # These labels are propagated to the pods generated for this workflow
    workflow: some-uid
    # The above label enables you to get a list of the pods associated with this workflow
    # via `oc get -lworkflow=some-uid`
spec:
  # Below are the most commonly used fields of the `spec` dictionary:
  # Workflows may be instantiated from a `package` or from an existing `instance`
  # the keys spec.package and spec.instance are mutually exclusive
  package:
    #the workflow package to be used as input to elaunch
    url: |
      https://github.com/mypackages/package.git 
             OR
      git@github.com/mypackages/package.git
    branch: master # the branch of the package
    mount: /mygit # Optional, if omitted it will be mounted in /mnt/package
    gitsecret: git-creds # Optional, needed only in the case where spec.package.url
    # refers to a private repo - will also be filled with default value
    # if the url is https://... the secret is expected to contain a key `oauth-token`
    # if the url is git@//... the secret is expected to contain the keys `ssh` and
    # `known_hosts`
    fromConfigMap: |
        name of ConfigMap that contains an entry `package.json`.
        The value of `package.json` is a dictionary which maps filepaths to the
        contents of the files. This JSON dictionary is extracted under the folder
        `$mount/lambda.package`. e.g the filePath `bin/hello.sh` refers to the file
        `$mount/lambda.package/bin/hello.sh`
  # The option below is useful when restarting a past workflow instance, don't forget
  # to also provide the additionalOption `--restart=<stage-index>` (mutually exclusive
  # with package)
  instance: "name of instance directory which is expected to exist in `working-volume`"

  # A list of volumes (similar to pod.spec.volumes)
  volumes:
    - name: <volume name>
      persistentVolumeClaim: # or any other volume type
        claimName: <pvc name>
  # A list of volumeMounts indicating locations that volumes will be mounted in the 
  # primary pod and the pods that st4sd-runtime-core creates to execute the workflow nodes
  # (similar to pod.spec.container.volumeMounts)
  volumeMounts:
    - name: <volume name>
      mountPath: /tmp/some/path/to/mount/the/volume/under

  # A list of absolute paths to be used as input files
  # OR a list of paths relative to inputDataVolume/DLF-dataset/S3-bucket 
  # can reference paths that volumes are mounted under (see volumes and volumeMounts)
  inputs: # Optional
    - /tmp/inputdir/field.conf
    - /tmp/inputdir/MOLECULE_LIBRARY
  # A list of absolute paths to be used as files containing user variables 
  # (override those defined in workflow, currently support up to 1)
  # can reference paths that volumes are mounted under (see volumes and volumeMounts)
  variables: # Optional
    - /tmp/inputdir/variables.conf
  # A list of absolute paths to be used as data files, they override those that come 
  # in the workflow data directory
  # OR a list of paths relative to inputDataVolume/DLF-dataset/S3-bucket 
  # can reference paths that volumes are mounted under (see volumes and volumeMounts))
  data: # Optional
    - /tmp/inputdir/CONTROL
  # A list of additional options to the workflow scheduler
  additionalOptions:  # Optional
    - "--platform=kubernetes"
    - "--log-level=15"
  
  # Remaining, optional configuration fields of the `spec` dictionary
  # (Optional) Configure the CPU and Memory resources, default values shown below
  resources:
    elaunchPrimary:  # Container that orchestrates the execution of the workflow
      cpu: "1000m"
      memory: "500Mi"
    gitFetch:  # Container that retrieves the workflow package from git
      cpu: "100m"
      memory: "200Mi"
    monitor:  # Side-car container that updates status field of this Workflow object
      cpu: "100m"
      memory: "200Mi"
  # ImagePullSecrets to use when pulling images for both the pod that orchestrates
  # the execution of the workflow as well as for the pods that the workflow generates
  imagePullSecrets: # Optional - can be filled in with default value
    - one
    - two
  # The volume to store the workflow instance, it's mounted under /tmp/workdir
  workingVolume: # Optional - can be filled in with default value
    name: working-volume
    persistentVolumeClaim:
      claimName: dummy-pvc1
  # A volume to provide input and data files to workflow instance
  # (deprecated, use volumes and volumeMounts)
  inputDataVolume: # Optional, it's mounted under /tmp/inputdir
    name: input-volume
    persistentVolumeClaim:
      claimName: pvc-name
  # Optionally fetch input/data files that reside in a S3 bucket
  s3BucketInput:
    # ST4SD Runtime Core supports Dataset Lifecycle Framework (DLF)
    #   see https://github.com/IBM/dataset-lifecycle-framework/ for more info
    dataset: "name of a DLF dataset object"
    # Alternatively, you can provide S3 information in the form of 4+1 `envVar` objects
    # see https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#envvar-v1-core
    accessKeyID: # envVar object
    secretAccessKey: # envVar object
    endpoint: # envVar object
    bucket: # envVar object
    region: # optional envVar object
  # Environment variables to provide to the containers of the main pod
  env:  # Optional
    # Spec: https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.19/#envvar-v1-core
    - name: <name of environment variable>
      value: <the value>
      # OR
      valueFrom:
        seretKeyRef: ... 
        fieldref: ...
        resourceFieldRef: ...
        configMapKeyRef: ...
  # Image of the workflow scheduler that will orchestrate the execution of the workflow
  # Optional - can be filled in with default value  
  image: res-st4sd-team-official-base-docker-local.artifactory.swg-devops.com/st4sd-runtime-k8s:latest
  # Image of the tool which will retrieve files from s3 bucket used as input
  s3FetchFilesImage: s3fetchfiles:latest # Optional - can be filled in with default value
  command: "elaunch.py" #optional, if omitted it would be elaunch.py
  debug: false  #optional, if set to true will just echo the command passed to the container
```
