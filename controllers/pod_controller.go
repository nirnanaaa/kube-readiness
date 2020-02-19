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

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	PodNotReadyErr = errors.New("pod is not ready, yet")
)

// PodReconciler reconciles a Pod object
type PodReconciler struct {
	client.Client
	Log            logr.Logger
	CloudSDK       cloud.SDK
	EndpointPodMap *readiness.EndpointPodMap
	IngressSet     *readiness.IngressSet
}

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=pods/status,verbs=get;update;patch

func (r *PodReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("pod", req.NamespacedName)
	ctx := context.Background()
	namespacedName := req.NamespacedName
	var pod corev1.Pod
	if err := r.Get(ctx, namespacedName, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	if !readiness.ReadinessGateEnabled(&pod) {
		return ctrl.Result{}, nil
	}

	status, _ := readiness.ReadinessConditionStatus(&pod)

	if status.Status == corev1.ConditionTrue {
		return ctrl.Result{}, nil
	}

	ingress, endpoint := r.IngressSet.FindByIP(pod.Status.PodIP)
	// TODO
	// fmt.Printf("%+v", r.IngressSet)
	if len(ingress.IngressEndpoints) == 0 {
		status.Status = corev1.ConditionUnknown
		if err := readiness.PatchPodStatus(r, ctx, &pod, status); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, errors.New("pod has not been assigned to an ingress")
	}

	healthy, err := r.CloudSDK.IsEndpointHealthy(ctx, ingress.LoadBalancer.Endpoints, pod.Status.PodIP, endpoint.Port)
	if err != nil {
		log.Error(err, "could not query target health")
		return ctrl.Result{}, err
	}

	if !healthy {
		log.Info("pod is not healthy, yet")
		status.Status = corev1.ConditionFalse
		if err := readiness.PatchPodStatus(r, ctx, &pod, status); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, PodNotReadyErr
	}

	status.Status = corev1.ConditionTrue
	return ctrl.Result{}, readiness.PatchPodStatus(r, ctx, &pod, status)
}

func (r *PodReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Pod{}).
		Complete(r)
}
