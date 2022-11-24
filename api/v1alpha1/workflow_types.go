/*
	Copyright IBM Inc. All Rights Reserved.

	SPDX-License-Identifier: Apache-2.0

	Authors:
	  Vassilis Vassiliadis
	  Yiannis Gkoufas
*/

package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WorkflowSpec defines the desired state of Workflow
type WorkflowSpec struct {
	// Image of workflow scheduler, leave blank to fill in with default option
	Image string `json:"image,omitempty"`

	// List of names of secret objects that the pods of workflow components can use
	// as imagePullSecrets
	// + optional
	ImagePullSecrets []string `json:"imagePullSecrets,omitempty"`
	// Package and Instance are mutually exclusive
	Package  *Gitrepo `json:"package,omitempty"`
	Instance string   `json:"instance,omitempty"`

	Debug bool `json:"debug,omitempty"`

	//if empty elaunch.py
	Command string `json:"command,omitempty"`

	// List of volumes that primary and minion pods will use
	// +optional
	Volumes []v1.Volume `json:"volumes,omitempty"`

	// List of volumemounts that primary and minion pods will use
	// +optional
	VolumeMounts []v1.VolumeMount `json:"volumeMounts,omitempty"`

	// Volume to host the workflow instance directory, leave blank to fill in with
	// default PersistentVolumeClaim
	WorkingVolume v1.Volume `json:"workingVolume,omitempty"`

	// Volume to mount under /tmp/inputdir (deprecated, will be automatically translated
	// to entries of Volumes and VolumeMounts)
	// +optional
	InputDataVolume *v1.Volume `json:"inputDataVolume,omitempty"`
	// Absolute paths to input files, files can reside in volumes
	// +optional
	Inputs []string `json:"inputs,omitempty"`

	// Absolute paths to variable files (currently support just 1 file). Variable file can reside in a volume.
	// +optional
	Variables []string `json:"variables,omitempty"`
	// Absolute paths to data files, files can reside in volumes. Identically named files are expected to already exist in the workflow package definition
	// +optional
	Data []string `json:"data,omitempty"`

	// Additional command-line arguments to orchestrator (e.g. ["--platform=openshift", "--log-level=15", "--discovererMonitorDir=/tmp/workdir/pod-reporter/update-files"]
	// +optional
	AdditionalOptions []string `json:"additionalOptions,omitempty"`

	// CPU and Memory resources for the 3 containers in the primary pod
	Resources *Resourcespec `json:"resources,omitempty"`

	// Environment variables to insert to all containers in primary pod
	// +optional
	Env []v1.EnvVar `json:"env,omitempty"`

	// Information for fetching inputs from a S3 bucket
	S3BucketInput     *S3BucketInfo `json:"s3BucketInput,omitempty"`
	S3FetchFilesImage string        `json:"s3FetchFilesImage,omitempty"`
}

type S3BucketInfo struct {
	Dataset         string          `json:"dataset,omitempty"`
	AccessKeyID     S3InputVariable `json:"accessKeyID,omitempty"`
	SecretAccessKey S3InputVariable `json:"secretAccessKey,omitempty"`
	Endpoint        S3InputVariable `json:"endpoint,omitempty"`
	Bucket          S3InputVariable `json:"bucket,omitempty"`
	Region          S3InputVariable `json:"region,omitempty"`
}

// +k8s:openapi-gen=true
type S3InputVariable struct {
	// Variable references $(VAR_NAME) are expanded
	// using the previous defined environment variables in the container and
	// any service environment variables. If a variable cannot be resolved,
	// the reference in the input string will be unchanged. The $(VAR_NAME)
	// syntax can be escaped with a double $$, ie: $$(VAR_NAME). Escaped
	// references will never be expanded, regardless of whether the variable
	// exists or not.
	// Defaults to "".
	// +optional
	Value string `json:"value,omitempty" protobuf:"bytes,2,opt,name=value"`
	// Source for the environment variable's value. Cannot be used if value is not empty.
	// +optional
	ValueFrom *v1.EnvVarSource `json:"valueFrom,omitempty" protobuf:"bytes,3,opt,name=valueFrom"`
}

// +k8s:openapi-gen=true
type Gitrepo struct {
	URL           string `json:"url,omitempty"`
	Branch        string `json:"branch,omitempty"`
	CommitId      string `json:"commitId,omitempty"`
	Mount         string `json:"mount,omitempty"`
	Gitsecret     string `json:"gitsecret,omitempty"`
	FromConfigMap string `json:"fromConfigMap,omitempty"`
	FromPath      string `json:"fromPath,omitempty"`
	WithManifest  string `json:"withManifest,omitempty"`
}

// +k8s:openapi-gen=true
type Resourcedefinition struct {
	Cpu    string `json:"cpu,omitempty"`
	Memory string `json:"memory,omitempty"`
}

// Defines Resource requests for containers in the primary pod of a workflow
// +k8s:openapi-gen=true
type Resourcespec struct {
	// Resource request for workflow orchestrator container
	// +optional
	ElaunchPrimary *Resourcedefinition `json:"elaunchPrimary,omitempty"`

	// Resource request for workflow monitoring side-container
	// +optional
	Monitor *Resourcedefinition `json:"monitor,omitempty"`

	// Resource request for git-clone init-container (deprecated)
	// +optional
	GitFetch *Resourcedefinition `json:"gitFetch,omitempty"`
}

// WorkflowStatus defines the observed state of Workflow
// +k8s:openapi-gen=true
type WorkflowStatus struct {
	Cost             string `json:"cost,omitempty"`
	Currentstage     string `json:"currentstage,omitempty"`
	Exitstatus       string `json:"exitstatus,omitempty"`
	Experimentstate  string `json:"experimentstate,omitempty"`
	Stageprogress    string `json:"stageprogress,omitempty"`
	Stagestate       string `json:"stagestate,omitempty"`
	Errordescription string `json:"errordescription,omitempty"`
	// +optional
	Stages        []string `json:"stages,omitempty"`
	Totalprogress string   `json:"totalprogress,omitempty"`
	Updated       string   `json:"updated,omitempty"`

	Outputfiles map[string]map[string]string `json:"outputfiles,omitempty"`
	Meta        string                       `json:"meta,omitempty"`
}

// DefaultWorkflowOptions holds default options to automatically generate parts of the
// workflow definition
// +k8s:openapi-gen=false
type DefaultWorkflowOptions struct {
	GitSyncImage            string   `json:"gitSyncImage,omitempty"`
	WorkflowMonitoringImage string   `json:"workflowMonitoringImage,omitempty"`
	S3FetchFilesImage       string   `json:"s3FetchFilesImage,omitempty"`
	FlowImage               string   `json:"flowImage,omitempty"`
	ImagePullSecrets        []string `json:"imagePullSecrets,omitempty"`
	WorkingVolume           string   `json:"workingVolume,omitempty"`
	GitSecret               string   `json:"gitSecret,omitempty"`
	GitSecretOAuth          string   `json:"gitSecretOAuth,omitempty"`
}

// ConsumableComputingConfig describes the contents of the `config.json` data entry of
// the consumable-computing-config ConfigMap
// +k8s:openapi-gen=false
type ConsumableComputingConfig struct {
	Image                   string   `json:"image,omitempty"`
	GitSecret               string   `json:"gitsecret,omitempty"`
	GitSecretOAuth          string   `json:"gitsecret-oauth,omitempty"`
	WorkingVolume           string   `json:"workingVolume,omitempty"`
	InputDataDir            string   `json:"inputdatadir,omitempty"`
	S3FetchFilesImage       string   `json:"s3-fetch-files-image,omitempty"`
	GitSyncImage            string   `json:"git-sync-image,omitempty"`
	WorkflowMonitoringImage string   `json:"workflow-monitoring-image,omitempty"`
	ImagePullSecrets        []string `json:"imagePullSecrets,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=workflows,shortName=wf
//+kubebuilder:printcolumn:name="age",type="string",JSONPath=".metadata.creationTimestamp",description="Age of the workflow instance"
//+kubebuilder:printcolumn:name="status",type="string",JSONPath=".status.experimentstate",description="Status of the workflow instance"
// Workflow is the Schema for the workflows API
type Workflow struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkflowSpec   `json:"spec,omitempty"`
	Status WorkflowStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:scope=Namespaced
// WorkflowList contains a list of Workflow
type WorkflowList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workflow `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workflow{}, &WorkflowList{})
}
