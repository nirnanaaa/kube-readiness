/*
Copyright 2019 Kube Readiness.

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

package main

import (
	"flag"
	"os"
	"sync"
	"time"

	"github.com/nirnanaaa/kube-readiness/controllers"
	"github.com/nirnanaaa/kube-readiness/pkg/cloud/aws"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = networkingv1beta1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = extensionsv1beta1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var region string
	var assumeRoleArn string
	var namespace string
	var enableLeaderElection bool
	var sdkCache bool
	var debug bool

	syncPeriod := 1 * time.Minute

	flag.StringVar(&metricsAddr, "metrics-addr", ":8081", "The address the metric endpoint binds to.")
	flag.StringVar(&assumeRoleArn, "aws-assume-role-arn", "", "A role that should be assumed from aws.")
	flag.StringVar(&region, "aws-region", "eu-west-1", "The AWS region to bind to.")
	flag.StringVar(&namespace, "namespace", "", "Namespace to listen on")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&debug, "debug", false,
		"Enable debug logging.")
	flag.BoolVar(&sdkCache, "sdk-cache", false,
		"enable the sdk cache (supported: AWS).")
	flag.Parse()

	ctrl.SetLogger(zap.Logger(debug))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:             scheme,
		MetricsBindAddress: metricsAddr,
		LeaderElection:     enableLeaderElection,
		SyncPeriod:         &syncPeriod,
		Namespace:          namespace,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	endpointPodMap := make(readiness.EndpointPodMap)
	awsSdk, err := aws.NewCloudSDK(region, assumeRoleArn, ctrl.Log.WithName("sdk").WithName("aws"), sdkCache)
	if err != nil {
		setupLog.Error(err, "unable to setup Cloud SDK", "component", "awsSDK")
		os.Exit(1)
	}
	endpointPodMutex := new(sync.RWMutex)
	serviceInfoMutex := new(sync.RWMutex)
	serviceInfoMap := make(readiness.ServiceInfoMap)

	if err = (&controllers.PodReconciler{
		Client:              mgr.GetClient(),
		Log:                 ctrl.Log.WithName("controllers").WithName("Pod"),
		EndpointPodMap:      endpointPodMap,
		EndpointPodMutex:    endpointPodMutex,
		ServiceInfoMap:      serviceInfoMap,
		ServiceInfoMapMutex: serviceInfoMutex,
		CloudSDK:            awsSdk,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pod")
		os.Exit(1)
	}
	serviceReconciler := &controllers.ServiceReconciler{
		Client:              mgr.GetClient(),
		ServiceInfoMap:      serviceInfoMap,
		EndpointPodMap:      endpointPodMap,
		Lock:                endpointPodMutex,
		ServiceInfoMapMutex: serviceInfoMutex,
		Log:                 ctrl.Log.WithName("controllers").WithName("Service"),
	}
	if err = (serviceReconciler).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Service")
		os.Exit(1)
	}
	if err = (&controllers.IngressReconciler{
		CloudSDK:            awsSdk,
		Client:              mgr.GetClient(),
		ServiceInfoMap:      serviceInfoMap,
		ServiceInfoMapMutex: serviceInfoMutex,
		ServiceReconciler:   serviceReconciler,
		Log:                 ctrl.Log.WithName("controllers").WithName("Ingress"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Ingress")
		os.Exit(1)
	}
	if err = (&controllers.EndpointsReconciler{
		Client:            mgr.GetClient(),
		EndpointPodMap:    endpointPodMap,
		ServiceReconciler: serviceReconciler,
		EndpointPodMutex:  endpointPodMutex,
		Log:               ctrl.Log.WithName("controllers").WithName("Endpoints"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Endpoints")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
