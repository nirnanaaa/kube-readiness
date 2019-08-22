package readiness

import (
	"context"
	"time"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness/alb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var dummyPod = &v1.Pod{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "test",
		Namespace: "default",
	},
	Spec: v1.PodSpec{
		Containers: []v1.Container{
			{
				Name: "test",
				Command: []string{
					"echo",
					"1",
				},
				Image: "busybox",
			},
		},
		ReadinessGates: []v1.PodReadinessGate{
			{
				ConditionType: alb.ReadinessGate,
			},
		},
	},
}

var podStatus = v1.PodStatus{
	Conditions: []v1.PodCondition{
		{
			Type:   alb.ReadinessGate,
			Status: v1.ConditionUnknown,
		},
	},
	PodIP: "123.244.255.254",
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

	Context("Pod Controller", func() {
		It("should add an ingress to check", func() {

			err := k8sClient.Create(context.TODO(), dummyPod)
			name := types.NamespacedName{
				Namespace: "default",
				Name:      "test",
			}
			var fetchedPod v1.Pod
			err = k8sClient.Get(context.TODO(), name, &fetchedPod)
			Expect(err).To(BeNil())
			fetchedPod.Status = podStatus
			err = k8sClient.Status().Update(context.TODO(), &fetchedPod)
			Expect(err).To(BeNil())

			ingressSet := controller.IngressSet.Ensure(name)
			ingressSet.IngressEndpoints.Insert(IngressEndpoint{
				IP:   "123.244.255.254",
				Port: 1234,
			})
			ingressSet.LoadBalancer.Hostname = "test1234"
			controller.SyncPod(name)
			var expectedPod v1.Pod

			Eventually(func() v1.ConditionStatus {
				err = k8sClient.Get(context.TODO(), name, &expectedPod)
				Expect(err).To(BeNil())
				return expectedPod.Status.Conditions[0].Status
			}, timeout, interval).Should(Equal(v1.ConditionTrue))
		})
	})
})
