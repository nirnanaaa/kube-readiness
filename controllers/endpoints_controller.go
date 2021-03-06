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
	"sync"

	"github.com/go-logr/logr"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

// EndpointsReconciler reconciles a Endpoints object
type EndpointsReconciler struct {
	client.Client
	EndpointPodMap    readiness.EndpointPodMap
	EndpointPodMutex  *sync.RWMutex
	ServiceReconciler *ServiceReconciler
	Log               logr.Logger
}

// +kubebuilder:rbac:groups=core,resources=endpoints,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=endpoints/status,verbs=get;update;patch

func (r *EndpointsReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	_ = r.Log.WithValues("endpoints", req.NamespacedName)
	r.ServiceReconciler.Reconcile(req)
	// your logic here

	return ctrl.Result{}, nil
}

func (r *EndpointsReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Endpoints{}).
		Complete(r)
}
