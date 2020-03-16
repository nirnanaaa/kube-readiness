package controllers

import (
	"context"
	"time"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness"
	"github.com/nirnanaaa/kube-readiness/pkg/readiness/alb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func patchPodStatus(pod *v1.Pod, status v1.PodStatus) (err error) {
	depPatch := client.MergeFrom(pod.DeepCopy())
	pod.Status = status
	return k8sClient.Status().Patch(context.TODO(), pod, depPatch)
}
func createDummyPodPod(optionalName *string) (pod *v1.Pod, name types.NamespacedName, rawName string) {
	podName := string(uuid.NewUUID())
	if optionalName != nil {
		podName = *optionalName
	}

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
					Ports: []v1.ContainerPort{
						{
							Name:          "http",
							ContainerPort: 80,
						},
					},
				},
			},
			ReadinessGates: []v1.PodReadinessGate{
				{
					ConditionType: alb.ReadinessGate,
				},
			},
		},
	}
	Expect(k8sClient.Create(context.TODO(), dummyPod)).To(Succeed())
	Expect(patchPodStatus(dummyPod, v1.PodStatus{
		PodIP:  "1.1.1.1",
		HostIP: "2.2.2.2",
	})).To(Succeed())
	return dummyPod, name, podName
}

var _ = Describe("Readiness Types", func() {
	const timeout = time.Second * 10
	const interval = time.Second * 1
	var name types.NamespacedName
	var pod *v1.Pod
	BeforeEach(func() {
		podReconciler.CloudSDK = &cloud.Fake{
			Unhealthy: false,
		}
	})
	AfterEach(func() {
		podReconciler.CloudSDK = &cloud.Fake{}
		k8sClient.Get(context.TODO(), name, pod)
		if pod != nil {
			k8sClient.Delete(context.TODO(), pod)
		}
	})

	Context("Pod Controller", func() {
		It("should not do anything for a pod, which does not have readiness gates enabled", func() {
			name = types.NamespacedName{
				Name:      "test",
				Namespace: "default",
			}
			dummyPod = &v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "default",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "test",
							Image: "nginx",
							Ports: []v1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(context.TODO(), dummyPod))
			Expect(patchPodStatus(dummyPod, v1.PodStatus{
				PodIP:  "1.1.1.1",
				HostIP: "2.2.2.2",
			})).To(Succeed())
			var fetchedPod v1.Pod
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), name, &fetchedPod)
			}, timeout, interval).ShouldNot(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(context.TODO(), name, &fetchedPod)
				Expect(err).To(BeNil())
				_, exists := readiness.ReadinessConditionStatus(&fetchedPod)
				return exists
			}, timeout, interval).Should(BeFalse())
		})
		It("should set the condition of the pod to unknown initially", func() {
			pod, name, _ = createDummyPodPod(nil)
			var fetchedPod v1.Pod
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), name, &fetchedPod)
			}, timeout, interval).ShouldNot(HaveOccurred())

			Eventually(func() v1.ConditionStatus {
				err := k8sClient.Get(context.TODO(), name, &fetchedPod)
				Expect(err).To(BeNil())
				validConditions, _ := readiness.ReadinessConditionStatus(&fetchedPod)
				return validConditions.Status
			}, timeout, interval).Should(Equal(v1.ConditionUnknown))
		})
		It("should not reconcile on an already running pod", func() {
			pod, name, _ = createDummyPodPod(nil)
			var fetchedPod v1.Pod
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), name, &fetchedPod)
			}, timeout, interval).Should(Succeed())
			Expect(patchPodStatus(&fetchedPod, v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:   alb.ReadinessGate,
						Status: v1.ConditionTrue,
					},
				},
			})).To(Succeed())
			Eventually(func() int64 {
				var pod v1.Pod
				Expect(k8sClient.Get(context.TODO(), name, &pod)).To(Succeed())
				return pod.Generation
			}).Should(Equal(fetchedPod.Generation))
		})
		It("should set the condition to ready when the cloudcontroller reports success", func() {
			podName := "pod-which-should-be-ready"

			serviceInfoMap.Add(name, readiness.IngressInfo{
				Name: "test",
				Endpoints: []*cloud.EndpointGroup{
					&cloud.EndpointGroup{Name: "test"},
				},
				Pods: []types.NamespacedName{{
					Namespace: "default",
					Name:      podName,
				}},
			})
			podReconciler.CloudSDK = &cloud.Fake{
				Unhealthy: false,
			}
			pod, name, _ = createDummyPodPod(&podName)
			var fetchedPod v1.Pod
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), name, &fetchedPod)
			}, timeout, interval).Should(Succeed())
			Expect(patchPodStatus(&fetchedPod, v1.PodStatus{
				Phase: v1.PodRunning,
			})).To(Succeed())
			Eventually(func() v1.ConditionStatus {
				var pod v1.Pod
				Expect(k8sClient.Get(context.TODO(), name, &pod)).To(Succeed())
				validConditions, _ := readiness.ReadinessConditionStatus(&pod)
				return validConditions.Status
			}, timeout, interval).Should(Equal(v1.ConditionTrue))
		})
		It("should set the condition to false when the cloudcontroller reports the target unhealthy", func() {
			podName := "somelocalpod"
			serviceInfoMap.Add(name, readiness.IngressInfo{
				Name: "test1",
				Endpoints: []*cloud.EndpointGroup{
					&cloud.EndpointGroup{Name: "test1"},
				},
				Pods: []types.NamespacedName{{
					Namespace: "default",
					Name:      podName,
				}},
			})
			podReconciler.CloudSDK = &cloud.Fake{
				Unhealthy: true,
			}
			pod, name, _ = createDummyPodPod(&podName)
			var fetchedPod v1.Pod
			Eventually(func() error {
				return k8sClient.Get(context.TODO(), name, &fetchedPod)
			}, timeout, interval).Should(Succeed())
			Expect(patchPodStatus(&fetchedPod, v1.PodStatus{
				Phase: v1.PodRunning,
			})).To(Succeed())

			Eventually(func() v1.ConditionStatus {
				var pod v1.Pod
				Expect(k8sClient.Get(context.TODO(), name, &pod)).To(Succeed())
				validConditions, _ := readiness.ReadinessConditionStatus(&pod)
				return validConditions.Status
			}, timeout, interval).Should(Equal(v1.ConditionFalse))
		})
		// It("should recheck periodically when a pod is not ready, yet", func() {

		// var fetchedPod v1.Pod
		// Eventually(func() error {
		// 	return k8sClient.Get(context.TODO(), name, &fetchedPod)
		// }, timeout, interval).ShouldNot(HaveOccurred())
		// 	var expectedPod v1.Pod

		// Eventually(func() v1.ConditionStatus {
		// 	err := k8sClient.Get(context.TODO(), name, &expectedPod)
		// 	Expect(err).To(BeNil())
		// 	validConditions, _ := readiness.ReadinessConditionStatus(&expectedPod)
		// 	return validConditions.Status
		// }, timeout, interval).Should(Equal(v1.ConditionUnknown))

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
