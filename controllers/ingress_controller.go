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
	"time"

	"github.com/go-logr/logr"
	"github.com/nirnanaaa/kube-readiness/controllers/utils"
	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// IngressReconciler reconciles a Ingress object
type IngressReconciler struct {
	client.Client
	CloudSDK            cloud.SDK
	Log                 logr.Logger
	ReadinessController *readiness.Controller
	ServiceInfoMapMutex *sync.RWMutex
	ServiceInfoMap      readiness.ServiceInfoMap
	ServiceReconciler   *ServiceReconciler
}

// +kubebuilder:rbac:groups=extensions,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=extensions,resources=ingresses/status,verbs=get;update;patch

func (r *IngressReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	_ = r.Log.WithValues("ingress", req.NamespacedName)
	ctx := context.Background()
	var ingress extensionsv1beta1.Ingress
	if err := r.Get(ctx, req.NamespacedName, &ingress); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	hostname, err := readiness.ExtractHostname(&ingress)
	if err != nil {
		return ctrl.Result{Requeue: true}, nil
	}
	endpointGroups, err := r.CloudSDK.GetEndpointGroupsByHostname(context.Background(), hostname)
	if err != nil {
		return ctrl.Result{}, err
	}
	utils.TraverseIngressBackends(&ingress, func(id utils.ServicePortID) bool {
		serviceName := types.NamespacedName{
			Namespace: id.Service.Namespace,
			Name:      id.Service.Name,
		}
		serviceInfo := readiness.IngressInfo{
			Endpoints: endpointGroups,
			Name:      hostname,
		}
		r.ServiceInfoMapMutex.Lock()
		r.ServiceInfoMap.Add(serviceName, serviceInfo)
		r.ServiceInfoMapMutex.Unlock()

		// log.Info("queueing service", "namespace", id.Service.Namespace, "name", id.Service.Name)
		r.ServiceReconciler.Reconcile(ctrl.Request{
			NamespacedName: serviceName,
		})
		return false
	})
	return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
}

func (r *IngressReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&extensionsv1beta1.Ingress{}).
		Complete(r)
}
