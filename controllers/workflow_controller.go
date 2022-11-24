/*
	Copyright IBM Inc. All Rights Reserved.

	SPDX-License-Identifier: Apache-2.0

	Authors:
	  Vassilis Vassiliadis
	  Yiannis Gkoufas
*/

package controllers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	st4sdv1alpha1 "github.ibm.com/st4sd/st4sd-runtime-k8s/api/v1alpha1"
)

const workflowFinalizer = "workflow.finalizer.st4sd.ibm.com"

// WorkflowReconciler reconciles a Workflow object
type WorkflowReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkflowReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&st4sdv1alpha1.Workflow{}).
		Complete(r)
}

func (r *WorkflowReconciler) finalizeWorkflow(reqLogger logr.Logger, wf *st4sdv1alpha1.Workflow) error {
	reqLogger.Info("Successfully finalized Workflow" + wf.ObjectMeta.Name)
	return nil
}

//+kubebuilder:rbac:groups=st4sd.ibm.com,resources=workflows,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=st4sd.ibm.com,resources=workflows/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=st4sd.ibm.com,resources=workflows/finalizers,verbs=update

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.11.0/pkg/reconcile
func (r *WorkflowReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	reqLogger := log.FromContext(ctx)

	// reqLogger.Info("Reconciling Workflow")

	// Fetch the Workflow instance
	instance := &st4sdv1alpha1.Workflow{}
	err := r.Get(ctx, req.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("Workflow resource not found. Ignoring since object must be deleted.")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get Workflow.")
		return ctrl.Result{}, err
	}

	// Check if the Workflow instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isMarkedToBeDeleted {
		if controllerutil.ContainsFinalizer(instance, workflowFinalizer) {
			// Run finalization logic for workflowFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeWorkflow(reqLogger, instance); err != nil {
				return ctrl.Result{}, err
			}

			// Remove workflowFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(instance, workflowFinalizer)
			err := r.Update(ctx, instance)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	// VV: Remove comments to add finalizer for Workflow objects
	/*
		 if !controllerutil.ContainsFinalizer(instance, workflowFinalizer) {
			controllerutil.AddFinalizer(instance, workflowFinalizer)
			err = r.Update(ctx, instance)
			if err != nil {
			 return ctrl.Result{}, err
			}
		 }*/

	if len(instance.Status.Updated) != 0 {
		/* reqLogger.Info("Workflow status has already been updated - this workflow has already " +
		"executed in the past, will not create a new pod.") */
		return ctrl.Result{}, nil
	}

	// Define a new Pod object
	pod, err := newPodForCR(r, instance)

	if err != nil {
		return ctrl.Result{}, err
	}

	// Set Workflow instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)

		if instance.ObjectMeta.Labels == nil {
			instance.ObjectMeta.Labels = make(map[string]string)
		}

		err := r.Client.Update(context.TODO(), instance)
		if err != nil {
			return ctrl.Result{}, err
		}

		err = r.Client.Create(context.TODO(), pod)
		if err != nil {
			return ctrl.Result{}, err
		}

		//TODO ignoring the errors for the time being
		randomHex, _ := randomHex(2)
		successfulPodCreationEvent := newEvent(instance.UID, instance.Namespace, instance.Name, "workflow-event-"+randomHex)
		err = r.Client.Create(context.TODO(), successfulPodCreationEvent)
		if err != nil {
			reqLogger.Error(err, "Error in creating event")
			return ctrl.Result{}, err
		}
		// reqLogger.Info("Event created successufly")

		// Pod created successfully - don't requeue
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	//config := ConfigurationWorkflow{
	//	Namespace: instance.Namespace,
	//	Workflow: instance.Spec,
	//}
	configYaml, err := yaml.Marshal(&pod)
	if err != nil {
		reqLogger.Error(err, "Error in marshalling the yaml, shouldnt happen")
		return ctrl.Result{}, err
	}

	configMap := newConfigMap(instance, string(configYaml))
	if err := controllerutil.SetControllerReference(instance, configMap, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}
	// Check if this Pod already exists

	foundConfigMap := &corev1.ConfigMap{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, foundConfigMap)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating config map", "Config.Namespace", configMap.Namespace, "Config.Name", configMap.Name)
		err = r.Client.Create(context.TODO(), configMap)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Pod created successfully - don't requeue
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Pod already exists - don't requeue
	// reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return ctrl.Result{}, nil
}

func newEvent(uid types.UID, namespace string, workflowname string, name string) *corev1.Event {
	return &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Action: "NewPodForWorkflow",
		InvolvedObject: corev1.ObjectReference{
			Kind:       "Workflow",
			Namespace:  namespace,
			Name:       workflowname,
			UID:        uid,
			APIVersion: "st4sd.ibm.com/v1alpha1",
		},
		Type:           "Warning", //tested so far Normal,Warning
		EventTime:      metav1.NowMicro(),
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
		Source: corev1.EventSource{
			Component: "workflow-controller",
		},
		ReportingInstance:   "workflow-controller-instance",
		Reason:              "NewPodForWorkflow",
		ReportingController: "workflow-controller-controller",
		Message:             "Successfully creating pod " + name + " in namespace " + namespace,
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func randomHex(n int) (string, error) {
	bytes := make([]byte, n)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}

func newConfigMap(cr *st4sdv1alpha1.Workflow, config string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name + "-config",
			Namespace: cr.Namespace,
		},
		// TODO VV: We could change this to `st4sd-k8s-conf.yaml` but that would break backwards compatibility with
		// existing workflow instances.
		Data: map[string]string{
			"flow-k8s-conf.yml": config,
		},
	}
}

func getDefaultValues(r *WorkflowReconciler, namespace string, configmap_name string) *st4sdv1alpha1.DefaultWorkflowOptions {
	logger := log.Log.WithName("getDefaultValues")

	var options = st4sdv1alpha1.DefaultWorkflowOptions{}

	// VV: Extract information from environment variables but then override that
	// with options extracted from the ConfigMap
	options.GitSyncImage = os.Getenv("GIT_SYNC_IMAGE")
	options.WorkflowMonitoringImage = os.Getenv("WORKFLOW_MONITORING_IMAGE")
	options.S3FetchFilesImage = os.Getenv("S3_FETCH_FILES_IMAGE")
	options.FlowImage = os.Getenv("FLOW_IMAGE")

	// VV: Get the consumable-computing-config ConfigMap and
	// try to extract default options from config.json
	configMap := corev1.ConfigMap{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: configmap_name, Namespace: namespace}, &configMap)

	if err != nil {
		logger.Info("Could not find consumable-computing-config ConfigMap", "err", err)
		return &options
	}

	configJSONStr, ok := configMap.Data["config.json"]

	if ok == false {
		logger.Info("Could not find config.json in consumable-computing-config ConfigMap", "err", err)
		return &options
	}

	configJSONbytes := []byte(configJSONStr)

	config := st4sdv1alpha1.ConsumableComputingConfig{}

	// VV: interestingly, yaml.Unmarshal does not decode all valid JSON strings
	err = json.Unmarshal(configJSONbytes, &config)

	if err != nil {
		logger.Info("Unable to unmarshal",
			"configJSONStr", configJSONStr, "err", err)
	}

	// VV: Got a valid Config dictionary - build the default options structure
	if len(config.Image) > 0 {
		options.FlowImage = config.Image
	}

	if len(config.GitSyncImage) > 0 {
		options.GitSyncImage = config.GitSyncImage
	}

	if len(config.S3FetchFilesImage) > 0 {
		options.S3FetchFilesImage = config.S3FetchFilesImage
	}

	if len(config.WorkflowMonitoringImage) > 0 {
		options.WorkflowMonitoringImage = config.WorkflowMonitoringImage
	}

	options.GitSecret = config.GitSecret
	options.GitSecretOAuth = config.GitSecretOAuth
	options.WorkingVolume = config.WorkingVolume
	options.ImagePullSecrets = config.ImagePullSecrets

	return &options
}

func rewrite_absolute_paths(paths []string, new_root string) []string {
	ret := make([]string, len(paths))

	for i, v := range paths {
		if !strings.HasPrefix(v, "/") {
			v = new_root + "/" + v
		}
		ret[i] = v
	}
	return ret
}

func newPodForCR(r *WorkflowReconciler, cr *st4sdv1alpha1.Workflow) (*corev1.Pod, error) {
	// reqLogger := log.Log.WithValues("workflow", cr.ObjectMeta.Name)

	var user, _ = strconv.ParseInt(os.Getenv("USER_ID"), 10, 64)
	var configmap_name = "st4sd-runtime-service"
	if cm_name := os.Getenv("CONFIGMAP_NAME"); cm_name != "" {
		configmap_name = cm_name
	}

	var options = getDefaultValues(r, cr.ObjectMeta.Namespace, configmap_name)

	type WorkflowSourceType string
	const (
		WorkflowSourcePackageHTTPS     = "https"
		WorkflowSourcePackageSSH       = "ssh"
		WorkflowSourcePackageConfigMap = "configMap"
		WorkflowSourceUnknown          = "unknown"
		WorkflowSourceInstance         = "instance"
		WorkflowSourcePackageFromPath  = "fromPath"
	)

	packageSource := WorkflowSourceUnknown

	if cr.Spec.Package != nil {
		if strings.HasPrefix(cr.Spec.Package.URL, "https") {
			packageSource = WorkflowSourcePackageHTTPS
		} else if strings.HasPrefix(cr.Spec.Package.URL, "git@") {
			packageSource = WorkflowSourcePackageSSH
		} else if len(cr.Spec.Package.FromPath) > 0 {
			packageSource = WorkflowSourcePackageFromPath
		}

		if len(cr.Spec.Package.FromConfigMap) > 0 {
			if packageSource != WorkflowSourceUnknown {
				return nil, fmt.Errorf("spec.package.fromConfigMap set but package is already configured " +
					"as " + packageSource)
			}
			packageSource = WorkflowSourcePackageConfigMap
		}
	}

	if len(cr.Spec.Instance) > 0 {
		if packageSource != WorkflowSourceUnknown {
			return nil, fmt.Errorf("spec.instance set but spec.package is set too (these fields are " +
				"mutually exclusive)")
		}

		packageSource = WorkflowSourceInstance
	}

	if packageSource == WorkflowSourceUnknown {
		return nil, fmt.Errorf("workflow object does not have a proper populated spec.package/instance")
	}

	// VV: Peek at the Workflow description and fill in the blanks
	logger := log.Log.WithName("injectValues")

	if cr.Spec.S3FetchFilesImage == "" {
		cr.Spec.S3FetchFilesImage = options.S3FetchFilesImage
		logger.Info("Setting", "cr.Spec.S3FetchFilesImage", options.S3FetchFilesImage)
	}

	if cr.Spec.Image == "" {
		cr.Spec.Image = options.FlowImage
		logger.Info("Setting", "cr.Spec.Image", options.FlowImage)
	}

	if (cr.Spec.Package != nil) && (cr.Spec.Package.Gitsecret == "") {
		if packageSource == WorkflowSourcePackageSSH {
			cr.Spec.Package.Gitsecret = options.GitSecret
			logger.Info("Setting", "cr.Spec.Package.Gitsecret", options.GitSecret)
		} else if packageSource == WorkflowSourcePackageHTTPS {
			cr.Spec.Package.Gitsecret = options.GitSecretOAuth
			logger.Info("Setting", "cr.Spec.Package.Gitsecret", options.GitSecretOAuth)
		}
	}

	if len(cr.Spec.ImagePullSecrets) == 0 {
		cr.Spec.ImagePullSecrets = options.ImagePullSecrets
		logger.Info("Setting", "cr.Spec.ImagePullSecrets", options.ImagePullSecrets)
	}

	if cr.Spec.WorkingVolume.Name == "" && options.WorkingVolume != "" {
		cr.Spec.WorkingVolume = v1.Volume{
			Name: "working-volume",
			VolumeSource: v1.VolumeSource{
				PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
					ClaimName: options.WorkingVolume,
					ReadOnly:  false,
				},
			},
		}
		logger.Info("Setting", "cr.Spec.WorkingVolume.PersistentVolumeClaim.Claimname",
			options.WorkingVolume)
	}

	// VV: Hard-coded variables go here (FIXME figure out how to deprecate those)
	workdir := "/tmp/workdir"
	rootDirConfigMapInputData := "/tmp/inputdir"
	rootDirS3InputData := "/tmp/s3-root-dir"
	inputdir := rootDirConfigMapInputData
	datadir := rootDirConfigMapInputData
	variabledir := rootDirConfigMapInputData

	// VV: Parse FSGROUP and use 5000 if nothing is provided
	// (PODS can read/write to PesistentVolumeClaim-folders using gid 5000)
	fsgroup := int64(5000)
	if t, ok := os.LookupEnv("FSGROUP"); ok {
		fsgroup, _ = strconv.ParseInt(t, 10, 64)
	}

	// VV: Parse GROU_ID and use 0 if nothing is provided
	// (best practices expect that PODS can read/write to image-folders using gid 0)
	group_id := int64(0)
	if t, ok := os.LookupEnv("GROUP_ID"); ok {
		group_id, _ = strconv.ParseInt(t, 10, 64)
	}

	labels := map[string]string{
		"workflow": cr.Name,
		"rest-uid": fmt.Sprint(cr.UID),
	}

	for k, v := range cr.ObjectMeta.Labels {
		labels[k] = v
	}

	if cr.Spec.S3BucketInput != nil {
		inputdir = rootDirS3InputData + "/input"
		datadir = rootDirS3InputData + "/data"
	}

	// VV: Now take care of Deprecated fields and ensure backwards compatibility

	// VV: First, handle Spec.InputDataVolume

	if cr.Spec.InputDataVolume != nil && len(cr.Spec.InputDataVolume.Name) > 0 {
		cr.Spec.Volumes = append(cr.Spec.Volumes, *cr.Spec.InputDataVolume)
		cr.Spec.VolumeMounts = append(cr.Spec.VolumeMounts, corev1.VolumeMount{
			Name:      cr.Spec.InputDataVolume.Name,
			MountPath: rootDirConfigMapInputData,
		})

		// VV: Finally, clear inputDataVolume
		cr.Spec.InputDataVolume = nil
	}

	// VV: Now rewrite Spec.Inputs, Spec.Data, and Spec.Variables to be absolute paths
	cr.Spec.Inputs = rewrite_absolute_paths(cr.Spec.Inputs, inputdir)
	cr.Spec.Variables = rewrite_absolute_paths(cr.Spec.Variables, variabledir)
	cr.Spec.Data = rewrite_absolute_paths(cr.Spec.Data, datadir)

	// VV: At this point, the workflow object has been migrated to latest Spec
	volumes := make([]v1.Volume, len(cr.Spec.Volumes))
	copy(volumes, cr.Spec.Volumes)

	volumeMountsPrimary := make([]v1.VolumeMount, len(cr.Spec.VolumeMounts))
	copy(volumeMountsPrimary, cr.Spec.VolumeMounts)

	volumes = append(volumes, corev1.Volume{
		Name: "config-volume",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: cr.Name + "-config",
				},
			},
		},
	})

	envVars := []corev1.EnvVar{}

	for _, v := range cr.Spec.Env {
		envVars = append(envVars, corev1.EnvVar{
			Name:      v.Name,
			Value:     v.Value,
			ValueFrom: v.ValueFrom,
		})
	}

	configVolumeMount := corev1.VolumeMount{
		Name: "config-volume",
		// TODO VV: We could change this to `st4sd-k8s-conf.yaml` but that would break backwards compatibility with
		// existing workflow instances.
		MountPath: "/etc/podinfo/flow-k8s-conf.yml",
		SubPath:   "flow-k8s-conf.yml",
	}

	volumeMountsPrimary = append(volumeMountsPrimary, configVolumeMount)

	volumes = append(volumes, cr.Spec.WorkingVolume)
	volumeMountsPrimary = append(volumeMountsPrimary, corev1.VolumeMount{
		Name:      cr.Spec.WorkingVolume.Name,
		MountPath: workdir,
	})

	if (cr.Spec.Package != nil) && len(cr.Spec.Package.Gitsecret) > 0 {
		var mode int32 = 288
		gitsecretsVolume := corev1.Volume{
			Name: "git-secrets-package",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName:  cr.Spec.Package.Gitsecret,
					DefaultMode: &mode,
				},
			},
		}
		volumes = append(volumes, gitsecretsVolume)
	}

	gitConfigVolume := corev1.Volume{
		Name: "git-sync-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "git-sync-config",
				},
			},
		},
	}
	volumes = append(volumes, gitConfigVolume)

	if packageSource == WorkflowSourcePackageConfigMap {
		lambdaConfigMapVolume := corev1.Volume{
			Name: cr.Spec.Package.FromConfigMap,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cr.Spec.Package.FromConfigMap,
					},
				},
			},
		}
		volumes = append(volumes, lambdaConfigMapVolume)
	}

	gitVolumeNamePackage := "git-sync-package"

	gitVolumePackage := corev1.Volume{
		Name: gitVolumeNamePackage,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	volumes = append(volumes, gitVolumePackage)

	tempVolumeName := "tmp-volume-name"

	tempVolume := corev1.Volume{
		Name: tempVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}

	volumes = append(volumes, tempVolume)

	packageMount := "/mnt/package"
	if (cr.Spec.Package != nil) && len(cr.Spec.Package.Mount) > 0 {
		packageMount = cr.Spec.Package.Mount
	}

	gitVolumeMountForPrimary := corev1.VolumeMount{
		Name:      gitVolumeNamePackage,
		MountPath: packageMount,
	}

	volumeMountsPrimary = append(volumeMountsPrimary, gitVolumeMountForPrimary)

	tempVolumeMountPrimary := corev1.VolumeMount{
		Name:      tempVolumeName,
		MountPath: "/tmp",
	}

	// VV: Use this to override UID/GID
	volMountGitSyncConfig := corev1.VolumeMount{
		Name:      "git-sync-config",
		MountPath: "/etc/passwd",
		SubPath:   "passwd",
	}

	volumeMountsPrimary = append(volumeMountsPrimary, tempVolumeMountPrimary)

	volumeMountsPrimary = append(volumeMountsPrimary, volMountGitSyncConfig)

	volumeMountsGitSyncPackageContainers := []corev1.VolumeMount{
		{
			Name:      gitVolumeNamePackage,
			MountPath: "/tmp/git",
		},
	}

	gitCloneOptions := []string{}

	branchName := ""

	if cr.Spec.Package != nil {
		branchName = cr.Spec.Package.Branch
	}

	volumeMountsGitSyncPackageContainers = append(volumeMountsGitSyncPackageContainers,
		volMountGitSyncConfig)

	overrideCommand := []string{}

	if packageSource == WorkflowSourcePackageSSH {
		gitCloneOptions = []string{
			"--one-time", "--depth=1", "--root=/tmp/git", "--submodules=recursive", "--exechook-command"}

		gitCloneOptions = append(gitCloneOptions, "--ssh", "--ssh-key-file=/etc/git-secret/ssh")

		if len(branchName) > 0 {
			gitCloneOptions = append(gitCloneOptions, "--branch="+branchName)
		} else if len(cr.Spec.Package.CommitId) > 0 {
			gitCloneOptions = append(gitCloneOptions, "--rev="+cr.Spec.Package.CommitId)
		}

		gitCloneOptions = append(gitCloneOptions, "--repo", cr.Spec.Package.URL)
	} else if packageSource == WorkflowSourcePackageHTTPS {
		u, err := url.Parse(cr.Spec.Package.URL)
		if err != nil {
			return nil, err
		}
		gitRoot := strings.Split(u.Path[1:], "/")[1]

		cmdGitInit := ""
		fullUrl := cr.Spec.Package.URL

		if len(cr.Spec.Package.Gitsecret) > 0 {
			fullUrl = "https://" + "`cat /etc/git-secret/oauth-token`@" + u.Host + "/" + u.Path[1:]
		}

		fullPath := "/tmp/git/" + gitRoot

		if len(cr.Spec.Package.CommitId) > 0 {
			cmdGitInit = "mkdir -p " + fullPath +
				" && cd " + fullPath +
				" && git init . " +
				" && git remote add origin " + fullUrl +
				" && git fetch --depth 1 origin " + cr.Spec.Package.CommitId +
				" && git checkout FETCH_HEAD"
		} else {
			cmdGitInit = "git clone --recurse-submodules --depth=1 " + fullUrl + " " + fullPath

			if len(branchName) > 0 {
				cmdGitInit += " --branch=" + branchName
			}
		}

		// VV: Remove the origin just to make sure that we do not expose the oauth token
		cmdGitInit += " && git -C " + fullPath + " submodule update --remote" +
			" &&  git -C " + fullPath + " remote remove origin"

		gitCloneOptions = []string{cmdGitInit}
		overrideCommand = []string{"/bin/sh", "-c"}
	} else if packageSource == WorkflowSourcePackageConfigMap {
		gitCloneOptions = []string{"/etc/flowir_package/package.json", "/tmp/git/"}
		volumeMountsGitSyncPackageContainers = append(volumeMountsGitSyncPackageContainers,
			corev1.VolumeMount{
				Name:      cr.Spec.Package.FromConfigMap,
				MountPath: "/etc/flowir_package",
			})
		overrideCommand = []string{"/bin/expand_package.py"}
	}

	//if there is a key it means private repo+ssh url
	if (cr.Spec.Package != nil) && len(cr.Spec.Package.Gitsecret) > 0 {
		volumeMountsGitSyncPackageContainers = append(volumeMountsGitSyncPackageContainers,
			corev1.VolumeMount{
				Name:      "git-secrets-package",
				MountPath: "/etc/git-secret",
			})
	}

	gitSyncResources := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100m"),
		corev1.ResourceMemory: resource.MustParse("200Mi"),
	}

	if cr.Spec.Resources != nil && cr.Spec.Resources.GitFetch != nil {
		if len(cr.Spec.Resources.GitFetch.Cpu) > 0 {
			gitSyncResources[corev1.ResourceCPU] = resource.MustParse(cr.Spec.Resources.GitFetch.Cpu)
		}
		if len(cr.Spec.Resources.GitFetch.Memory) > 0 {
			gitSyncResources[corev1.ResourceMemory] = resource.MustParse(cr.Spec.Resources.GitFetch.Memory)
		}
	}

	initContainerPackage := corev1.Container{
		Name:  "git-sync-package",
		Image: options.GitSyncImage,
		Resources: corev1.ResourceRequirements{
			Limits:   gitSyncResources,
			Requests: gitSyncResources,
		},
		ImagePullPolicy: corev1.PullAlways,
		Args:            gitCloneOptions,
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:  &user,
			RunAsGroup: &user,
		},
		VolumeMounts: volumeMountsGitSyncPackageContainers,
	}

	if len(overrideCommand) > 0 {
		initContainerPackage.Command = overrideCommand
	}

	wfMonitorResources := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("100m"),
		corev1.ResourceMemory: resource.MustParse("200Mi"),
	}

	if cr.Spec.Resources != nil && cr.Spec.Resources.Monitor != nil {
		if len(cr.Spec.Resources.Monitor.Cpu) > 0 {
			wfMonitorResources[corev1.ResourceCPU] = resource.MustParse(cr.Spec.Resources.Monitor.Cpu)
		}
		if len(cr.Spec.Resources.Monitor.Memory) > 0 {
			wfMonitorResources[corev1.ResourceMemory] = resource.MustParse(cr.Spec.Resources.Monitor.Memory)
		}
	}

	volumeMountsMonitor := []corev1.VolumeMount{
		{
			Name:      cr.Spec.WorkingVolume.Name,
			MountPath: "/tmp/workdir",
		},
		tempVolumeMountPrimary,
		configVolumeMount,
	}

	volumeMountsMonitor = append(volumeMountsMonitor, corev1.VolumeMount{
		Name:      "git-sync-config",
		MountPath: "/etc/passwd",
		SubPath:   "passwd",
	})

	monitorElaunchContainer := corev1.Container{
		Name:  "monitor-elaunch-container",
		Image: options.WorkflowMonitoringImage,
		Env:   envVars,
		Lifecycle: &corev1.Lifecycle{
			PreStop: &corev1.LifecycleHandler{
				Exec: &corev1.ExecAction{
					Command: []string{
						"/bin/sh",
						"-c",
						"echo Hello from the postStart handler",
						"sleep 10",
					},
				},
			},
		},
		Resources: corev1.ResourceRequirements{
			Limits:   wfMonitorResources,
			Requests: wfMonitorResources,
		},
		ImagePullPolicy: corev1.PullAlways,
		VolumeMounts:    volumeMountsMonitor,
	}

	command := []string{"elaunch.py"}
	if len(cr.Spec.Command) > 0 {
		command = []string{cr.Spec.Command}
	}

	for _, v := range cr.Spec.Inputs {
		command = append(command, "-i", v)
	}
	for _, v := range cr.Spec.Variables {
		command = append(command, "-a", v)
	}
	for _, v := range cr.Spec.Data {
		command = append(command, "-d", v)
	}

	command = append(command, cr.Spec.AdditionalOptions...)

	fullPath := ""

	if packageSource == WorkflowSourcePackageHTTPS ||
		packageSource == WorkflowSourcePackageSSH {
		fullPath = path.Join(packageMount, path.Base(cr.Spec.Package.URL))
	} else if packageSource == WorkflowSourcePackageConfigMap {
		fullPath = path.Join(packageMount, "lambda.package")
	} else if packageSource == WorkflowSourceInstance {
		fullPath = path.Join(workdir, cr.Spec.Instance)
		// VV: Automatically generate the INSTANCE_DIR_NAME env variable
		envVars = append(envVars, corev1.EnvVar{
			Name:  "INSTANCE_DIR_NAME",
			Value: cr.Spec.Instance})
	}

	if len(cr.Spec.Package.WithManifest) > 0 {
		if fullPath == "" || filepath.IsAbs(cr.Spec.Package.WithManifest) {
			command = append(command, "--manifest", cr.Spec.Package.WithManifest)
		} else {
			command = append(command, "--manifest", path.Join(fullPath, cr.Spec.Package.WithManifest))
		}
	}

	if len(cr.Spec.Package.FromPath) > 0 {
		fromPath := cr.Spec.Package.FromPath
		if fullPath == "" || filepath.IsAbs(fromPath) {
			fullPath = fromPath
		} else {
			fullPath = path.Join(fullPath, cr.Spec.Package.FromPath)
		}
	}

	command = append(command, fullPath)

	elaunchResources := corev1.ResourceList{
		corev1.ResourceCPU:    resource.MustParse("1000m"),
		corev1.ResourceMemory: resource.MustParse("500Mi"),
	}

	if cr.Spec.Resources != nil && cr.Spec.Resources.ElaunchPrimary != nil {
		if len(cr.Spec.Resources.ElaunchPrimary.Cpu) > 0 {
			elaunchResources[corev1.ResourceCPU] = resource.MustParse(cr.Spec.Resources.ElaunchPrimary.Cpu)
		}
		if len(cr.Spec.Resources.ElaunchPrimary.Memory) > 0 {
			elaunchResources[corev1.ResourceMemory] = resource.MustParse(cr.Spec.Resources.ElaunchPrimary.Memory)
		}
	}

	initcontainers := []corev1.Container{}

	if packageSource == WorkflowSourcePackageHTTPS ||
		packageSource == WorkflowSourcePackageSSH ||
		packageSource == WorkflowSourcePackageConfigMap {
		initcontainers = append(initcontainers, initContainerPackage)
	}

	if cr.Spec.S3BucketInput != nil {
		s3FetchFilesImage := ""
		if cr.Spec.S3FetchFilesImage != "" {
			s3FetchFilesImage = cr.Spec.S3FetchFilesImage
		} else {
			s3FetchFilesImage = options.S3FetchFilesImage
		}

		volMountS3Fetch := corev1.VolumeMount{
			Name: "s3-fetch", MountPath: rootDirS3InputData}

		volumeMountsS3FetchFiles := []corev1.VolumeMount{
			volMountS3Fetch, volMountGitSyncConfig,
		}
		volumeMountsPrimary = append(volumeMountsPrimary, volMountS3Fetch)

		s3FetchCommand := []string{}
		s3EnvVars := []corev1.EnvVar{{Name: "ROOT_OUTPUT", Value: rootDirS3InputData}}

		if cr.Spec.S3BucketInput.Dataset != "" {
			s3Keys := map[string]string{
				"S3_ACCESS_KEY_ID":     "accessKeyID",
				"S3_SECRET_ACCESS_KEY": "secretAccessKey",
				"S3_ENDPOINT":          "endpoint",
				"S3_BUCKET":            "bucket",
				"S3_REGION":            "region",
			}

			for envName, keyName := range s3Keys {
				s3EnvVars = append(s3EnvVars,
					corev1.EnvVar{
						Name: envName,
						ValueFrom: &corev1.EnvVarSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: cr.Spec.S3BucketInput.Dataset,
								}, Key: keyName}}})
			}
		} else {
			s3EnvVars = append(s3EnvVars,
				corev1.EnvVar{
					Name:      "S3_ACCESS_KEY_ID",
					Value:     cr.Spec.S3BucketInput.AccessKeyID.Value,
					ValueFrom: cr.Spec.S3BucketInput.AccessKeyID.ValueFrom,
				}, corev1.EnvVar{
					Name:      "S3_SECRET_ACCESS_KEY",
					Value:     cr.Spec.S3BucketInput.SecretAccessKey.Value,
					ValueFrom: cr.Spec.S3BucketInput.SecretAccessKey.ValueFrom,
				}, corev1.EnvVar{
					Name:      "S3_ENDPOINT",
					Value:     cr.Spec.S3BucketInput.Endpoint.Value,
					ValueFrom: cr.Spec.S3BucketInput.Endpoint.ValueFrom,
				}, corev1.EnvVar{
					Name:      "S3_BUCKET",
					Value:     cr.Spec.S3BucketInput.Bucket.Value,
					ValueFrom: cr.Spec.S3BucketInput.Bucket.ValueFrom,
				},
				corev1.EnvVar{
					Name:      "S3_REGION",
					Value:     cr.Spec.S3BucketInput.Region.Value,
					ValueFrom: cr.Spec.S3BucketInput.Region.ValueFrom,
				},
			)
		}

		// VV: Input/Data files which are expected to be retrieved via s3
		// are expected to be stored under the rootDirS3InputData folder
		s3_dir_input := rootDirS3InputData + "/input/"
		s3_dir_data := rootDirS3InputData + "/data/"

		for _, v := range cr.Spec.Inputs {
			if strings.HasPrefix(v, s3_dir_input) {
				s3_path := v[len(s3_dir_input):]
				complex := st4sdv1alpha1.SplitPathToSourcePathAndTargetName(s3_path)
				s3FetchCommand = append(s3FetchCommand, "-i", complex.SourcePath)
			}
		}
		for _, v := range cr.Spec.Data {
			if strings.HasPrefix(v, s3_dir_data) {
				s3_path := v[len(s3_dir_data):]
				complex := st4sdv1alpha1.SplitPathToSourcePathAndTargetName(s3_path)
				s3FetchCommand = append(s3FetchCommand, "-d", complex.SourcePath)
			}
		}

		// VV: Use a S3 bucket to fetch input and data files instead of a ConfigMap
		s3FetchFiles := corev1.Container{
			Name:            "s3-fetch",
			Image:           s3FetchFilesImage,
			Args:            s3FetchCommand,
			Env:             s3EnvVars,
			ImagePullPolicy: corev1.PullAlways,
			VolumeMounts:    volumeMountsS3FetchFiles,
			Resources: corev1.ResourceRequirements{
				Limits:   gitSyncResources,
				Requests: gitSyncResources,
			},
			WorkingDir: "/workdir",
			SecurityContext: &corev1.SecurityContext{
				RunAsUser:  &user,
				RunAsGroup: &user,
			},
		}

		initcontainers = append(initcontainers, s3FetchFiles)

		volumes = append(volumes, corev1.Volume{
			Name:         "s3-fetch",
			VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
	}

	primarycontainer := corev1.Container{
		Name:            "elaunch-primary",
		Image:           cr.Spec.Image,
		Env:             envVars,
		ImagePullPolicy: corev1.PullAlways,
		VolumeMounts:    volumeMountsPrimary,
		Resources: corev1.ResourceRequirements{
			Limits:   elaunchResources,
			Requests: elaunchResources,
		},
		WorkingDir: workdir,
	}
	// this is a hack to just display what the arguments would be without running anything
	if cr.Spec.Debug == true {
		primarycontainer.Command = append([]string{"echo"}, command...)
	} else {
		primarycontainer.Command = command
	}

	containers := []corev1.Container{
		primarycontainer, monitorElaunchContainer,
	}

	imagePullSecrets := []corev1.LocalObjectReference{}
	for _, v := range cr.Spec.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: v})
	}

	var rootuser, _ = strconv.ParseInt(os.Getenv("USER_ID"), 10, 64)

	var terminationSeconds int64
	terminationSeconds = 600

	// VV: Default name of ServiceAccount is "workflow-operator" but override it using the
	// env-var SERVICE_ACCOUNT_NAME
	serviceAccountName := "workflow-operator"
	if t, ok := os.LookupEnv("SERVICE_ACCOUNT_NAME"); ok {
		serviceAccountName = t
	}

	// reqLogger.Info("checking service account " + serviceAccountName)

	var podSpec = corev1.PodSpec{
		//giving the same permissions to the pod
		//uses a pre-created one
		ServiceAccountName:            serviceAccountName,
		RestartPolicy:                 corev1.RestartPolicyNever,
		Volumes:                       volumes,
		TerminationGracePeriodSeconds: &terminationSeconds,
		Containers:                    containers,
		InitContainers:                initcontainers,
		SecurityContext: &corev1.PodSecurityContext{
			FSGroup:    &fsgroup,
			RunAsUser:  &rootuser,
			RunAsGroup: &group_id,
		},
	}
	if len(imagePullSecrets) != 0 {
		podSpec.ImagePullSecrets = imagePullSecrets
	}
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cr.Name,
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: podSpec,
	}

	// podJSON, _ := json.Marshal(&pod)
	// reqLogger.Info(string(podJSON))

	return &pod, nil
}
