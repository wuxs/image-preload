/*
Copyright 2022.

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

package controllers

import (
	"context"
	"errors"
	"fmt"
	jobv1 "github.com/wuxs/image-preload/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"time"

	//	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ImagePullJobReconciler reconciles a ImagePullJob object
type ImagePullJobReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=job.wuxs.vip,resources=imagepulljobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=job.wuxs.vip,resources=imagepulljobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=job.wuxs.vip,resources=imagepulljobs/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core,resources=pods/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ImagePullJob object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *ImagePullJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	var err error
	ipj := &jobv1.ImagePullJob{}
	err = r.Get(ctx, req.NamespacedName, ipj)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			logger.Info("ImagePullJob not found")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "get ImagePullJob error")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if ipj.Spec.Images == nil {
		logger.Error(err, "invalid image list")
		return ctrl.Result{}, errors.New("invalid image list")
	}

	if ipj.DeletionTimestamp != nil {
		logger.Info("ImagePullJob has been deleted")
		CleanPods(r.Client, ipj.Status.Tasks)
		return ctrl.Result{}, err
	}

	switch ipj.Status.Status {
	case jobv1.Pending:
		scheduledResult := ctrl.Result{RequeueAfter: time.Minute} // 保存以便别处复用
		allFinished := true
		for node, task := range ipj.Status.Tasks {
			pod := &corev1.Pod{}
			key := types.NamespacedName{Namespace: task.NameSpace, Name: task.Name}
			err = r.Get(ctx, key, pod)
			if err != nil {
				logger.Error(err, "get Pod error", "node", node)
				continue
			}
			logger.Info("get pod success", "pod.status", pod.Status)
			if len(pod.Spec.Containers) != len(pod.Status.ContainerStatuses) {
				logger.Info("pod is pending, skip", "containers", len(pod.Spec.Containers), "status", len(pod.Status.ContainerStatuses))
				allFinished = false
				continue
			}

			taskStatus := jobv1.Finished
			for _, s := range pod.Status.ContainerStatuses {
				if s.State.Terminated != nil {
					ipj.Status.Tasks[node].ImageStatus[s.Image] = jobv1.Finished
					logger.Info("container terminated", "pod", pod.Name, "reason", s.State.Terminated.Reason)
				} else {
					ipj.Status.Tasks[node].ImageStatus[s.Image] = jobv1.Pending
					logger.Info("container pending", "pod", pod.Name, "container", s.Image)
					taskStatus = jobv1.Pending
				}
			}
			task.Status = taskStatus
			if taskStatus == jobv1.Pending {
				allFinished = false
			} else {
				logger.Info("task finished, delete pod", "status", taskStatus)
			}
		}
		if allFinished {
			logger.Info("ImagePullJob finished", "status", ipj.Status)
			ipj.Status.Status = jobv1.Finished
		}
		err = r.Status().Update(ctx, ipj)
		if err != nil {
			logger.Error(err, "update Status error")
			return ctrl.Result{}, err
		}
		return scheduledResult, err
	case jobv1.Finished:
		CleanPods(r.Client, ipj.Status.Tasks)
		err = r.Delete(ctx, ipj)
		if err != nil {
			logger.Error(err, "delete ImagePullJob error")
		}
	default:
		logger.Info("ImagePullJob status", "status", ipj.Status)
		nodes := make([]string, 0)
		if ipj.Spec.Selector.Names != nil {
			nodes = append(nodes, ipj.Spec.Selector.Names...)
		}
		if ipj.Spec.Selector.MatchLabels != nil {
			nodeList := &corev1.NodeList{}
			err = r.List(ctx, nodeList, client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(ipj.Spec.Selector.MatchLabels)})
			if err != nil {
				logger.Error(err, "get NodeList error", "labels", ipj.Spec.Selector.MatchLabels)
				return ctrl.Result{}, err
			}
			for _, n := range nodeList.Items {
				nodes = append(nodes, n.Name)
			}
		}

		tasks := make(map[string]*jobv1.Task)
		for _, node := range nodes {
			pod := CreatePob(node, ipj.Spec.Images)
			err = r.Create(ctx, pod)
			if err != nil {
				logger.Error(err, "create Pod error")
				return ctrl.Result{}, err
			}
			if err = controllerutil.SetControllerReference(ipj, pod, r.Scheme); err != nil {
				return reconcile.Result{}, err
			}
			task := &jobv1.Task{
				Name:        pod.Name,
				NameSpace:   pod.Namespace,
				Status:      jobv1.Pending,
				ImageStatus: map[string]jobv1.Status{},
			}
			for _, image := range ipj.Spec.Images {
				task.ImageStatus[image] = jobv1.Pending
			}
			tasks[node] = task
		}
		ipj.Status.Status = jobv1.Pending
		ipj.Status.Nodes = UniqueArray(nodes)
		ipj.Status.Tasks = tasks
		err = r.Status().Update(ctx, ipj)
		if err != nil {
			logger.Error(err, "update Status error")
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ImagePullJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&jobv1.ImagePullJob{}).
		Complete(r)
}

func UniqueArray(old []string) []string {
	result := make([]string, 0)
	unique := make(map[string]struct{})
	for _, s := range old {
		unique[s] = struct{}{}
	}
	for k := range unique {
		result = append(result, k)
	}
	return result
}

func GetPodName(node string) string {
	return node
}

func CreatePob(node string, images []string) *corev1.Pod {
	containers := make([]corev1.Container, len(images))
	for i, image := range images {
		c := corev1.Container{
			Name:            fmt.Sprintf("image-%d", i),
			Image:           image,
			Command:         []string{"echo", "hello"},
			ImagePullPolicy: corev1.PullIfNotPresent,
		}
		containers[i] = c
	}
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GetPodName(node),
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			Containers:    containers,
			RestartPolicy: corev1.RestartPolicyNever,
			NodeName:      node,
			Tolerations: []corev1.Toleration{
				{
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
		},
	}
}

func CleanPods(c client.Client, tasks map[string]*jobv1.Task) {
	for _, task := range tasks {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      task.Name,
				Namespace: task.NameSpace,
			},
		}
		_ = c.Delete(context.Background(), pod)
	}
}
