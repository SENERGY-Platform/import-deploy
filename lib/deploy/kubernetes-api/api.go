package kubernetes_api

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"
	"sync"

	"github.com/SENERGY-Platform/import-deploy/lib/baggage"
	"github.com/SENERGY-Platform/import-deploy/lib/config"
	"github.com/SENERGY-Platform/import-deploy/lib/deploy"
	"github.com/SENERGY-Platform/import-deploy/lib/model"
	"github.com/SENERGY-Platform/import-deploy/lib/util"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	autoscaling_k8s_io_v1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	autoscaler "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type k8s struct {
	clientset           *kubernetes.Clientset
	autoscalerClientset *autoscaler.Clientset
	config              config.Config
}

func New(config config.Config) (client deploy.DeploymentClient, err error) {
	var restConfig *rest.Config
	if config.KubeConfig != "" {
		kubeconfig := config.KubeConfig
		configOverrides := &clientcmd.ConfigOverrides{}
		configLoadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configLoadingRules.ExplicitPath = kubeconfig
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(configLoadingRules, configOverrides)
		restConfig, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build kube config: %v", err)
		}
	} else {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build in-cluster kube config: %v", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %v", err)
	}
	autoscalerClientSet, err := autoscaler.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &k8s{clientset, autoscalerClientSet, config}, nil
}

func (this *k8s) CreateContainer(ctx context.Context, name string, image string, env map[string]string, restart bool, userid string, importTypeId string, bag map[string]string) (id string, err error) {
	ctx, cf := util.GetTimeoutContext(ctx)
	defer cf()
	container := getContainer(name, image, env)
	labels := map[string]string{
		"user":         userid,
		"importId":     name,
		"importTypeId": strings.ReplaceAll(importTypeId, ":", "_"),
	}
	podLabels := baggage.AddLabels(ctx, maps.Clone(labels), bag)
	var targetRef *autoscalingv1.CrossVersionObjectReference
	if restart {
		// create deployment
		deployment := getDeployment(name, labels, podLabels, container, this.config.ImagePullSecrets)
		_, err = this.clientset.AppsV1().Deployments(this.config.RancherNamespaceId).Create(ctx, deployment, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to create deployment: %v", err)
		}
		targetRef = &autoscalingv1.CrossVersionObjectReference{
			Kind: "Deployment",
			Name: name,
		}
	} else {
		// create job
		job := getJob(name, podLabels, container)
		_, err = this.clientset.BatchV1().Jobs(this.config.RancherNamespaceId).Create(ctx, job, metav1.CreateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to create job: %v", err)
		}
		targetRef = &autoscalingv1.CrossVersionObjectReference{
			Kind:       "Job",
			APIVersion: "batch/v1",
			Name:       name,
		}
	}
	// create vpa
	recreate := autoscaling_k8s_io_v1.UpdateModeRecreate
	_, err = this.autoscalerClientset.AutoscalingV1().VerticalPodAutoscalers(this.config.RancherNamespaceId).Create(ctx, &autoscaling_k8s_io_v1.VerticalPodAutoscaler{
		ObjectMeta: metav1.ObjectMeta{
			Name: name + "-vpa",
		},
		Spec: autoscaling_k8s_io_v1.VerticalPodAutoscalerSpec{
			TargetRef: targetRef,
			UpdatePolicy: &autoscaling_k8s_io_v1.PodUpdatePolicy{
				UpdateMode: &recreate,
			},
			ResourcePolicy: &autoscaling_k8s_io_v1.PodResourcePolicy{
				ContainerPolicies: []autoscaling_k8s_io_v1.ContainerResourcePolicy{
					{
						ContainerName: "*",
						MaxAllowed: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1"),
							corev1.ResourceMemory: resource.MustParse("4000Mi"),
						},
					},
				},
			},
		},
	}, metav1.CreateOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to create vpa: %v", err)
	}

	return name, nil
}

func (this *k8s) UpdateContainer(ctx context.Context, id string, name string, image string, env map[string]string, restart bool, userid string, importTypeId string, existingRestart bool, bag map[string]string) (newId string, err error) {
	if existingRestart != restart || !restart {
		// cannot update restart policy, need to delete and recreate
		// cannot update jobs, need to delete and recreate
		err = this.RemoveContainer(ctx, id)
		if err != nil {
			return newId, err
		}
		return this.CreateContainer(ctx, name, image, env, restart, userid, importTypeId, bag)
	} else {
		// update deployment
		ctx, cf := util.GetTimeoutContext(ctx)
		defer cf()
		container := getContainer(name, image, env)
		labels := map[string]string{
			"user":         userid,
			"importId":     name,
			"importTypeId": strings.ReplaceAll(importTypeId, ":", "_"),
		}
		deployment := getDeployment(name, labels, baggage.AddLabels(ctx, maps.Clone(labels), bag), container, this.config.ImagePullSecrets)
		_, err = this.clientset.AppsV1().Deployments(this.config.RancherNamespaceId).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to update deployment: %v", err)
		}
		return name, nil
	}
}

func (this *k8s) RemoveContainer(ctx context.Context, id string) (err error) {
	ctx, cf := util.GetTimeoutContext(ctx)
	defer cf()
	var supErr error
	mux := sync.Mutex{}
	wg := sync.WaitGroup{}

	// delete deployment
	wg.Go(func() {
		err = this.clientset.AppsV1().Deployments(this.config.RancherNamespaceId).Delete(ctx, id, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			mux.Lock()
			supErr = errors.Join(supErr, fmt.Errorf("failed to delete deployment: %v", err))
			mux.Unlock()
		}
	})

	// delete job
	wg.Go(func() {
		err = this.clientset.BatchV1().Jobs(this.config.RancherNamespaceId).Delete(ctx, id, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			mux.Lock()
			supErr = errors.Join(supErr, fmt.Errorf("failed to delete job: %v", err))
			mux.Unlock()
		}
	})

	// delete service (was created for legacy imports created with rancher-2 API)
	wg.Go(func() {
		err = this.clientset.CoreV1().Services(this.config.RancherNamespaceId).Delete(ctx, id, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			mux.Lock()
			supErr = errors.Join(supErr, fmt.Errorf("failed to delete service: %v", err))
			mux.Unlock()
		}
	})

	// delete vpa
	wg.Go(func() {
		err = this.autoscalerClientset.AutoscalingV1().VerticalPodAutoscalers(this.config.RancherNamespaceId).Delete(ctx, id+"-vpa", metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			mux.Lock()
			supErr = errors.Join(supErr, fmt.Errorf("failed to delete vpa: %v", err))
			mux.Unlock()
		}
	})

	// delete vpa-checkpoint
	wg.Go(func() {
		err = this.autoscalerClientset.AutoscalingV1().VerticalPodAutoscalerCheckpoints(this.config.RancherNamespaceId).Delete(ctx, id+"-vpa-"+id, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			mux.Lock()
			supErr = errors.Join(supErr, fmt.Errorf("failed to delete vpa-checkpoint: %v", err))
			mux.Unlock()
		}
	})

	// delete completed pods
	wg.Go(func() {
		err = this.clientset.CoreV1().Pods(this.config.RancherNamespaceId).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: "importId=" + id,
		})
		if err != nil && !apierrors.IsNotFound(err) {
			mux.Lock()
			supErr = errors.Join(supErr, fmt.Errorf("failed to delete pods %s: %v", id, err))
			mux.Unlock()
		}
	})

	wg.Wait()
	return supErr
}

func (this *k8s) ContainerExists(ctx context.Context, id string, restart *bool) (exists bool, err error) {
	ctx, cf := util.GetTimeoutContext(ctx)
	defer cf()
	found := false
	if restart == nil || *restart { // default => restart enabled => deployment
		deployment, err2 := this.clientset.AppsV1().Deployments(this.config.RancherNamespaceId).Get(ctx, id, metav1.GetOptions{})
		err = err2
		found = deployment != nil
	} else {
		job, err2 := this.clientset.BatchV1().Jobs(this.config.RancherNamespaceId).Get(ctx, id, metav1.GetOptions{})
		err = err2
		found = job != nil
	}
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return found, nil
}

func (this *k8s) GetContainerStatus(ctx context.Context, id string, restart *bool) (status model.InstanceStatus, err error) {
	ctx, cf := util.GetTimeoutContext(ctx)
	defer cf()
	if restart == nil || *restart { // default => restart enabled => deployment
		deployment, err := this.clientset.AppsV1().Deployments(this.config.RancherNamespaceId).Get(ctx, id, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return notFoundStatus(), nil
			}
			return status, fmt.Errorf("failed to get deployment: %v", err)
		}
		return deploymentStatus(deployment), nil
	}
	job, err := this.clientset.BatchV1().Jobs(this.config.RancherNamespaceId).Get(ctx, id, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return notFoundStatus(), nil
		}
		return status, fmt.Errorf("failed to get job: %v", err)
	}
	return jobStatus(job), nil
}

func (this *k8s) GetContainerStatuses(ctx context.Context) (statuses map[string]model.InstanceStatus, err error) {
	ctx, cf := util.GetTimeoutContext(ctx)
	defer cf()
	statuses = map[string]model.InstanceStatus{}
	var supErr error
	mux := sync.Mutex{}
	wg := sync.WaitGroup{}

	wg.Go(func() {
		deployments, err := this.clientset.AppsV1().Deployments(this.config.RancherNamespaceId).List(ctx, metav1.ListOptions{})
		mux.Lock()
		defer mux.Unlock()
		if err != nil {
			supErr = errors.Join(supErr, fmt.Errorf("failed to list deployments: %v", err))
			return
		}
		for _, deployment := range deployments.Items {
			statuses[deployment.Name] = deploymentStatus(&deployment)
		}
	})

	wg.Go(func() {
		jobs, err := this.clientset.BatchV1().Jobs(this.config.RancherNamespaceId).List(ctx, metav1.ListOptions{})
		mux.Lock()
		defer mux.Unlock()
		if err != nil {
			supErr = errors.Join(supErr, fmt.Errorf("failed to list jobs: %v", err))
			return
		}
		for _, job := range jobs.Items {
			statuses[job.Name] = jobStatus(&job)
		}
	})

	wg.Wait()
	if supErr != nil {
		return nil, supErr
	}
	return statuses, nil
}

func (this *k8s) Disconnect() (err error) {
	return nil
}

func notFoundStatus() model.InstanceStatus {
	return model.InstanceStatus{Running: false, Transitioning: false, Message: "not deployed"}
}

func deploymentStatus(deployment *appsv1.Deployment) model.InstanceStatus {
	status := model.InstanceStatus{
		Running:       deployment.Status.AvailableReplicas > 0 && deployment.Status.UnavailableReplicas == 0,
		Transitioning: deployment.Status.UnavailableReplicas > 0,
	}
	// surface the reason of the most recent condition that is not satisfied
	for _, condition := range deployment.Status.Conditions {
		if condition.Status != corev1.ConditionTrue && condition.Message != "" {
			status.Message = condition.Message
			break
		}
	}
	return status
}

func jobStatus(job *batchv1.Job) model.InstanceStatus {
	status := model.InstanceStatus{
		Running: job.Status.Active > 0,
	}
	switch {
	case job.Status.Failed > 0:
		status.Message = "failed"
	case job.Status.Succeeded > 0:
		status.Message = "completed"
	case job.Status.Active == 0:
		status.Transitioning = true
		status.Message = "pending"
	}
	for _, condition := range job.Status.Conditions {
		if condition.Status == corev1.ConditionTrue && condition.Message != "" {
			status.Message = condition.Message
			break
		}
	}
	return status
}

func getContainer(name string, image string, env map[string]string) corev1.Container {
	envs := []corev1.EnvVar{}
	for k, v := range env {
		envs = append(envs, corev1.EnvVar{
			Name:  k,
			Value: v,
		})
	}
	return corev1.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: "Always",
		Env:             envs,
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("500m"),
				corev1.ResourceMemory: resource.MustParse("512Mi"),
			},
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("100m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		},
	}
}

// getDeployment keeps the selector on the plain labels and puts the baggage-carrying
// ones on the pod template only: a deployment's selector is immutable, so a workload
// created before an instance carried baggage could no longer be updated, and the log
// aggregation reads pod labels anyway.
func getDeployment(name string, labels map[string]string, podLabels map[string]string, container corev1.Container, imagePullSecrets []string) *appsv1.Deployment {
	pullsecrets := []corev1.LocalObjectReference{}
	for _, secret := range imagePullSecrets {
		pullsecrets = append(pullsecrets, corev1.LocalObjectReference{Name: secret})
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: podLabels,
				},
				Spec: corev1.PodSpec{
					Containers:       []corev1.Container{container},
					ImagePullSecrets: pullsecrets,
				},
			},
		},
	}
}

func getJob(name string, labels map[string]string, container corev1.Container) *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers:    []corev1.Container{container},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}
}
