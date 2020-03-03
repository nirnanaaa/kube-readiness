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
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	corev1 "k8s.io/api/core/v1"
)

// ServiceReconciler reconciles a Service object
type ServiceReconciler struct {
	client.Client
	ServiceInfoMapMutex *sync.RWMutex
	ServiceInfoMap      readiness.ServiceInfoMap
	EndpointPodMap      readiness.EndpointPodMap
	Log                 logr.Logger
	Lock                *sync.RWMutex
}

// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=services/status,verbs=get;update;patch

func (r *ServiceReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("service", req.NamespacedName)
	var service corev1.Service
	if err := r.Get(ctx, req.NamespacedName, &service); err != nil {
		if apierrors.IsNotFound(err) {
			r.ServiceInfoMapMutex.Lock()
			defer r.ServiceInfoMapMutex.Unlock()
			r.ServiceInfoMap.Remove(req.NamespacedName)
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}
	var endpoints corev1.Endpoints
	if err := r.Get(ctx, req.NamespacedName, &endpoints); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// get pods for service
	pods := r.getPodsForService(&endpoints)
	r.ServiceInfoMapMutex.Lock()
	defer r.ServiceInfoMapMutex.Unlock()
	serviceInfo, ok := r.ServiceInfoMap[req.NamespacedName]
	if !ok {
		return ctrl.Result{Requeue: true}, nil
	}
	serviceInfo.Pods = pods

	r.ServiceInfoMap.Add(req.NamespacedName, serviceInfo)

	return ctrl.Result{}, nil
}

func (r *ServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Complete(r)
}

func (r *ServiceReconciler) getPodsForService(endpoints *corev1.Endpoints) []types.NamespacedName {
	r.Lock.RLock()
	defer r.Lock.RUnlock()
	var podNames []types.NamespacedName
	for _, endpointSubset := range endpoints.Subsets {
		for _, port := range endpointSubset.Ports {
			for _, address := range endpointSubset.NotReadyAddresses {
				name, err := getPodFromEndpointMap(r.EndpointPodMap, address, port)
				if err != nil {
					continue
				}
				podNames = append(podNames, name)
			}
			for _, address := range endpointSubset.Addresses {
				name, err := getPodFromEndpointMap(r.EndpointPodMap, address, port)
				if err != nil {
					continue
				}
				podNames = append(podNames, name)

			}
		}
	}
	return podNames
}

func getPodFromEndpointMap(endpointMap readiness.EndpointPodMap, address corev1.EndpointAddress, port corev1.EndpointPort) (podName types.NamespacedName, err error) {
	endpoint := readiness.IngressEndpoint{
		IP:   address.IP,
		Port: port.Port,
	}
	if pod, ok := endpointMap[endpoint]; ok {
		return pod, nil
	}
	return types.NamespacedName{}, errors.New("no pod for service found")
}
