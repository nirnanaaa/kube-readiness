package readiness

import (
	"context"

	"github.com/nirnanaaa/kube-readiness/pkg/readiness/alb"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ReadinessConditionStatus(pod *v1.Pod) (condition v1.PodCondition, exists bool) {
	emptyPodCondition := v1.PodCondition{
		Type: alb.ReadinessGate,
	}
	if pod == nil {
		return emptyPodCondition, false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == alb.ReadinessGate {
			return condition, true
		}
	}
	return emptyPodCondition, false
}

func SetReadinessConditionStatus(pod *v1.Pod, condition v1.PodCondition) {
	if pod == nil {
		return
	}
	for i, cond := range pod.Status.Conditions {
		if cond.Type == alb.ReadinessGate {
			pod.Status.Conditions[i] = condition
			return
		}
	}
	pod.Status.Conditions = append(pod.Status.Conditions, condition)
}

func ReadinessGateEnabled(pod *v1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, cond := range pod.Spec.ReadinessGates {
		if cond.ConditionType == alb.ReadinessGate {
			return true
		}
	}
	return false
}

func PatchPodStatus(c client.Client, ctx context.Context, pod *v1.Pod, condition v1.PodCondition) error {
	depPatch := client.MergeFrom(pod.DeepCopy())
	SetReadinessConditionStatus(pod, condition)
	return c.Status().Patch(ctx, pod, depPatch)
}
