package readiness

import (
	"github.com/nirnanaaa/kube-readiness/pkg/readiness/alb"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

var _ = Describe("Pod Gates", func() {
	Context("ReadinessGateEnabled", func() {
		It("should not be enabled for an empty pod", func() {
			Expect(ReadinessGateEnabled(nil)).To(BeFalse())
		})
		It("should not be enabled for a pod without the gate set", func() {
			pod := &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{},
				},
			}
			Expect(ReadinessGateEnabled(pod)).To(BeFalse())
		})
		It("should be enabled for a pod with the gate set", func() {
			pod := &v1.Pod{
				Spec: v1.PodSpec{
					ReadinessGates: []v1.PodReadinessGate{
						{
							ConditionType: alb.ReadinessGate,
						},
					},
				},
			}
			Expect(ReadinessGateEnabled(pod)).To(BeTrue())
		})
	})
	Context("SetReadinessConditionStatus", func() {
		It("should work for an empty pod", func() {
			newCondition := v1.PodCondition{
				Type:   alb.ReadinessGate,
				Status: v1.ConditionFalse,
			}
			SetReadinessConditionStatus(nil, newCondition)
		})
		It("should work for a set pod", func() {
			status := v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:   alb.ReadinessGate,
						Status: v1.ConditionTrue,
					},
				},
			}
			newCondition := v1.PodCondition{
				Type:   alb.ReadinessGate,
				Status: v1.ConditionFalse,
			}
			pod := &v1.Pod{Status: status}
			SetReadinessConditionStatus(pod, newCondition)
			Expect(pod.Status.Conditions[0].Status).To(Equal(v1.ConditionFalse))
		})
		It("should work for a non existing condition", func() {
			status := v1.PodStatus{
				Conditions: []v1.PodCondition{},
			}
			newCondition := v1.PodCondition{
				Type:   alb.ReadinessGate,
				Status: v1.ConditionFalse,
			}
			pod := &v1.Pod{Status: status}
			SetReadinessConditionStatus(pod, newCondition)
			Expect(pod.Status.Conditions[0].Status).To(Equal(v1.ConditionFalse))
		})
	})
	Context("ReadinessConditionStatus", func() {
		It("should get a new status", func() {
			By("giving an empty pod")
			_, exists := ReadinessConditionStatus(nil)
			Expect(exists).To(BeFalse())

			By("setting an existing pod without a condition")
			_, exists = ReadinessConditionStatus(&v1.Pod{})
			Expect(exists).To(BeFalse())
		})
		It("should get an existing status", func() {
			status := v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:   alb.ReadinessGate,
						Status: v1.ConditionTrue,
					},
				},
			}
			condition, exists := ReadinessConditionStatus(&v1.Pod{Status: status})
			Expect(exists).To(BeTrue())
			Expect(condition.Status).To(Equal(v1.ConditionTrue))

			status = v1.PodStatus{
				Conditions: []v1.PodCondition{
					{
						Type:   alb.ReadinessGate,
						Status: v1.ConditionFalse,
					},
				},
			}
			By("conditionFalse")
			condition, exists = ReadinessConditionStatus(&v1.Pod{Status: status})
			Expect(exists).To(BeTrue())
			Expect(condition.Status).To(Equal(v1.ConditionFalse))
		})
	})
})
