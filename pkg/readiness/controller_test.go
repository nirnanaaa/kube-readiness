package readiness

import (
	"context"
	"time"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
)

var backend = extensionsv1beta1.IngressBackend{
	ServiceName: "test",
	ServicePort: intstr.FromString("http"),
}

var dummyIngress = &extensionsv1beta1.Ingress{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
	},
	Spec: extensionsv1beta1.IngressSpec{
		Rules: []extensionsv1beta1.IngressRule{
			{
				Host: "test",
				IngressRuleValue: extensionsv1beta1.IngressRuleValue{
					HTTP: &extensionsv1beta1.HTTPIngressRuleValue{
						Paths: []extensionsv1beta1.HTTPIngressPath{
							{
								Path:    "/test",
								Backend: backend,
							},
						},
					},
				},
			},
		},
	},
}

var ingressStatus = extensionsv1beta1.IngressStatus{
	LoadBalancer: v1.LoadBalancerStatus{
		Ingress: []v1.LoadBalancerIngress{
			{
				Hostname: "testabc",
			},
		},
	},
}

var dummyService = &v1.Service{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
	},
	Spec: v1.ServiceSpec{
		Ports: []v1.ServicePort{
			{
				Name:       "http",
				Protocol:   v1.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromInt(80),
			},
		},
		// Ports: []v1.ServicePorts{},
	},
}

var dummyEndpoint = &v1.Endpoints{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
	},
	Subsets: []v1.EndpointSubset{
		{
			Addresses: []v1.EndpointAddress{
				{
					IP: "123.244.255.254",
				},
			},
			NotReadyAddresses: []v1.EndpointAddress{
				{
					IP: "123.244.255.254",
				},
			},
			Ports: []v1.EndpointPort{
				{
					Name: "http",
					Port: 80,
				},
			},
		},
		// Ports: []v1.ServicePorts{},
	},
}

var _ = Describe("Readiness Types", func() {
	const timeout = time.Second * 5
	const interval = time.Second * 1
	var controller *Controller
	var stopCh chan struct{}
	BeforeEach(func() {
		controller = NewController(k8sClient)
		controller.Log = ctrl.Log.WithName("controllers").WithName("Readiness")
		controller.CloudSDK = &cloud.Fake{}
		stopCh = make(chan struct{})
		go controller.Run(stopCh)
	})

	AfterEach(func() {
		close(stopCh)
	})

	Context("Ingress Controller", func() {
		It("should add an ingress to check", func() {

			err := k8sClient.Create(context.TODO(), dummyIngress)
			Expect(err).To(BeNil())
			err = k8sClient.Create(context.TODO(), dummyService)
			Expect(err).To(BeNil())
			err = k8sClient.Create(context.TODO(), dummyEndpoint)
			Expect(err).To(BeNil())

			name := types.NamespacedName{
				Namespace: "default",
				Name:      "test",
			}
			var fetchedIngress extensionsv1beta1.Ingress
			err = k8sClient.Get(context.TODO(), name, &fetchedIngress)
			Expect(err).To(BeNil())
			fetchedIngress.Status = ingressStatus
			err = k8sClient.Status().Update(context.TODO(), &fetchedIngress)
			Expect(err).To(BeNil())

			controller.SyncIngress(name)
			Eventually(func() IngressData {
				return controller.IngressSet[name]
			}, timeout, interval).ShouldNot(BeNil())
		})
	})
})
