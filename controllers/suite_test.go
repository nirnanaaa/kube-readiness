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
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var k8sManager ctrl.Manager
var testEnv *envtest.Environment
var endpointPodMap readiness.EndpointPodMap
var serviceInfoMap readiness.ServiceInfoMap
var cloudsdk *cloud.Fake
var podReconciler *PodReconciler
var serviceReconciler *ServiceReconciler
var endpointsReconciler *EndpointsReconciler
var ingressReconciler *IngressReconciler
var responseDataMap map[string]bool

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{envtest.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	cloudsdk = &cloud.Fake{}
	endpointPodMap = make(readiness.EndpointPodMap)
	serviceInfoMap = make(readiness.ServiceInfoMap)

	By("bootstrapping test environment")
	t := true
	if os.Getenv("TEST_USE_EXISTING_CLUSTER") == "true" {
		testEnv = &envtest.Environment{
			UseExistingCluster: &t,
		}
	} else {
		testEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
		}
		testEnv.ControlPlaneStartTimeout = 20 * time.Second
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = extensionsv1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	endpointLock := new(sync.RWMutex)
	serviceLock := new(sync.RWMutex)

	// +kubebuilder:scaffold:scheme

	k8sManager, err = ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())
	k8sClient = k8sManager.GetClient() //.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(k8sClient).ToNot(BeNil())

	serviceReconciler = &ServiceReconciler{
		Client:              k8sClient,
		Log:                 ctrl.Log.WithName("controllers").WithName("ServiceScope"),
		EndpointPodMap:      endpointPodMap,
		Lock:                endpointLock,
		ServiceInfoMap:      serviceInfoMap,
		ServiceInfoMapMutex: serviceLock,
	}
	err = (serviceReconciler).SetupWithManager(k8sManager)

	Expect(err).ToNot(HaveOccurred())

	endpointsReconciler = &EndpointsReconciler{
		Client:            k8sClient,
		Log:               ctrl.Log.WithName("controllers").WithName("ServiceScope"),
		ServiceReconciler: serviceReconciler,
	}
	err = (endpointsReconciler).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	ingressReconciler = &IngressReconciler{
		Client:              k8sClient,
		Log:                 ctrl.Log.WithName("controllers").WithName("ServiceScope"),
		ServiceInfoMap:      serviceInfoMap,
		ServiceInfoMapMutex: serviceLock,
		ServiceReconciler:   serviceReconciler,
	}
	err = (ingressReconciler).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())
	podReconciler = &PodReconciler{
		Client:              k8sClient,
		Log:                 ctrl.Log.WithName("controllers").WithName("PodScope"),
		CloudSDK:            cloudsdk,
		EndpointPodMap:      endpointPodMap,
		EndpointPodMutex:    new(sync.RWMutex),
		ServiceInfoMap:      serviceInfoMap,
		ServiceInfoMapMutex: serviceLock,
	}
	err = (podReconciler).SetupWithManager(k8sManager)

	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	gexec.KillAndWait(5 * time.Second)
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
