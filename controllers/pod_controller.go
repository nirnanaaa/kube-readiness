/*
Copyright 2019 Kube Readiness Maintainers.

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
	"sync"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	PodNotReadyErr = errors.New("pod is not ready, yet")
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log                 logr.Logger
	CloudSDK            cloud.SDK
	EndpointPodMutex    *sync.RWMutex
	ServiceInfoMapMutex *sync.RWMutex
	EndpointPodMap      readiness.EndpointPodMap
	ServiceInfoMap      readiness.ServiceInfoMap
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch

func (r *PodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("pod", req.NamespacedName)
	ctx := context.Background()
	namespacedName := req.NamespacedName
	var pod corev1.Pod
	if err := r.Get(ctx, namespacedName, &pod); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if pod.DeletionTimestamp != nil {
		log.Info("pod is in deletion, not reconciling")
		return ctrl.Result{}, nil
	}
	if pod.Status.PodIP == "" {
		return ctrl.Result{Requeue: true}, nil
	}
	r.writePodMapEndpoint(&pod, namespacedName)
	if !readiness.ReadinessGateEnabled(&pod) {
		return ctrl.Result{}, nil
	}

	status, _ := readiness.ReadinessConditionStatus(&pod)
	r.ServiceInfoMapMutex.Lock()
	defer r.ServiceInfoMapMutex.Unlock()
	serviceInfo, err := r.ServiceInfoMap.GetServiceInfoForPod(req.NamespacedName)
	if err != nil {
		status.Status = corev1.ConditionUnknown
		if err := readiness.PatchPodStatus(r, ctx, &pod, status); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	ports := getContainerPortsForPod(&pod)
	healthy, err := r.CloudSDK.IsEndpointHealthy(ctx, serviceInfo.Endpoints, pod.Status.PodIP, ports)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !healthy {
		log.Info("pod is not healthy, yet")
		status.Status = corev1.ConditionFalse
		status.LastProbeTime = metav1.Now()
		if err := readiness.PatchPodStatus(r, ctx, &pod, status); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}
	if status.Status == corev1.ConditionTrue {
		return ctrl.Result{}, nil
	}
	log.Info("pod transitioned to state ready")
	status.Status = corev1.ConditionTrue
	status.LastTransitionTime = metav1.Now()
	return ctrl.Result{}, readiness.PatchPodStatus(r, ctx, &pod, status)
}

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}

func (r *PodReconciler) writePodMapEndpoint(pod *corev1.Pod, namespacedName types.NamespacedName) {
	r.EndpointPodMutex.Lock()
	for _, port := range getContainerPortsForPod(pod) {
		endpoint := readiness.IngressEndpoint{
			IP:   pod.Status.PodIP,
			Port: port,
		}
		r.EndpointPodMap[endpoint] = namespacedName
	}
	r.EndpointPodMutex.Unlock()
}

func getContainerPortsForPod(pod *corev1.Pod) []int32 {
	var ports []int32
	for _, container := range pod.Spec.Containers {
		for _, port := range container.Ports {
			ports = append(ports, port.ContainerPort)
		}
	}
	return ports
}
