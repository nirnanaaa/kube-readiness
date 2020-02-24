package controllers

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
	"k8s.io/apimachinery/pkg/util/uuid"
)

var dummyPod *v1.Pod

var podStatus = v1.PodStatus{
	Conditions: []v1.PodCondition{
		{
			Type:   alb.ReadinessGate,
			Status: v1.ConditionUnknown,
		},
	},
}

var _ = Describe("Readiness Types", func() {
	const timeout = time.Second * 10
	const interval = time.Second * 1
	var podName string
	var name types.NamespacedName

	BeforeEach(func() {
		podName = string(uuid.NewUUID())

		name = types.NamespacedName{
			Namespace: "default",
			Name:      podName,
		}
		dummyPod = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: "default",
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name:  "test",
						Image: "nginx",
					},
				},
				ReadinessGates: []v1.PodReadinessGate{
					{
						ConditionType: alb.ReadinessGate,
					},
				},
			},
		}
		err := k8sClient.Create(context.TODO(), dummyPod)
		Expect(err).NotTo(HaveOccurred())
	})
	AfterEach(func() {
		podReconciler.CloudSDK = &cloud.Fake{}
		var fetchedPod v1.Pod
		err := k8sClient.Get(context.TODO(), name, &fetchedPod)
		Expect(err).NotTo(HaveOccurred())
		k8sClient.Delete(context.TODO(), &fetchedPod)
	})

	Context("Pod Controller", func() {
		// It("should recheck periodically when a pod is not ready, yet", func() {

		// 	var fetchedPod v1.Pod
		// 	Eventually(func() error {
		// 		return k8sClient.Get(context.TODO(), name, &fetchedPod)
		// 	}, timeout, interval).ShouldNot(HaveOccurred())
		// 	var expectedPod v1.Pod

		// 	Eventually(func() v1.ConditionStatus {
		// 		err := k8sClient.Get(context.TODO(), name, &expectedPod)
		// 		Expect(err).To(BeNil())
		// 		validConditions, _ := readiness.ReadinessConditionStatus(&expectedPod)
		// 		return validConditions.Status
		// 	}, timeout, interval).Should(Equal(v1.ConditionUnknown))

		// })
		// It("should have falsey health conditions for non-ready endpoints", func() {
		// 	healthy := false
		// 	podReconciler.CloudSDK = &cloud.Fake{Healthy: &healthy}
		// 	var fetchedPod v1.Pod
		// 	Eventually(func() error {
		// 		return k8sClient.Get(context.TODO(), name, &fetchedPod)
		// 	}, timeout, interval).ShouldNot(HaveOccurred())

		// 	Eventually(func() string {
		// 		err := k8sClient.Get(context.TODO(), name, &fetchedPod)
		// 		Expect(err).NotTo(HaveOccurred())
		// 		return fetchedPod.Status.PodIP
		// 	}, timeout, interval).ShouldNot(BeEmpty())

		// 	Eventually(func() error {
		// 		err := k8sClient.Get(context.TODO(), name, &fetchedPod)
		// 		Expect(err).NotTo(HaveOccurred())
		// 		fetchedPod.Status = podStatus
		// 		return k8sClient.Status().Update(context.TODO(), &fetchedPod)
		// 	}, timeout, interval).ShouldNot(HaveOccurred())

		// 	ingressSetting := ingressSet.Ensure(name)
		// 	ingressSetting.IngressEndpoints.Insert(readiness.IngressEndpoint{
		// 		IP:   fetchedPod.Status.PodIP,
		// 		Port: 1234,
		// 	})
		// 	ingressSetting.LoadBalancer.Hostname = "test1234"
		// 	var expectedPod v1.Pod

		// 	Eventually(func() v1.ConditionStatus {
		// 		err := k8sClient.Get(context.TODO(), name, &expectedPod)
		// 		Expect(err).To(BeNil())
		// 		validConditions, _ := readiness.ReadinessConditionStatus(&expectedPod)
		// 		return validConditions.Status
		// 	}, timeout, interval).Should(Equal(v1.ConditionFalse))

		// })
		// It("should re-evaluate an already ready pod", func() {
		// 	var fetchedPod v1.Pod
		// 	Eventually(func() error {
		// 		return k8sClient.Get(context.TODO(), name, &fetchedPod)
		// 	}, timeout, interval).ShouldNot(HaveOccurred())

		// 	Eventually(func() string {
		// 		err := k8sClient.Get(context.TODO(), name, &fetchedPod)
		// 		Expect(err).NotTo(HaveOccurred())
		// 		return fetchedPod.Status.PodIP
		// 	}, timeout, interval).ShouldNot(BeEmpty())

		// 	Eventually(func() error {
		// 		err := k8sClient.Get(context.TODO(), name, &fetchedPod)
		// 		Expect(err).NotTo(HaveOccurred())
		// 		fetchedPod.Status = podStatus
		// 		return k8sClient.Status().Update(context.TODO(), &fetchedPod)
		// 	}, timeout, interval).ShouldNot(HaveOccurred())

		// 	ingressSetting := ingressSet.Ensure(name)
		// 	ingressSetting.IngressEndpoints.Insert(readiness.IngressEndpoint{
		// 		IP:   fetchedPod.Status.PodIP,
		// 		Port: 1234,
		// 	})
		// 	ingressSetting.LoadBalancer.Hostname = "test1234"
		// 	var expectedPod v1.Pod

		// 	Eventually(func() v1.ConditionStatus {
		// 		err := k8sClient.Get(context.TODO(), name, &expectedPod)
		// 		Expect(err).To(BeNil())
		// 		validConditions, _ := readiness.ReadinessConditionStatus(&expectedPod)
		// 		return validConditions.Status
		// 	}, timeout, interval).Should(Equal(v1.ConditionTrue))
		// })
	})
})
