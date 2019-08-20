package readiness

import (
	"context"

	"github.com/nirnanaaa/kube-readiness/pkg/readiness/alb"
	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func readinessConditionStatus(pod *v1.Pod) (condition v1.PodCondition, exists bool) {
	if pod == nil {
		return v1.PodCondition{}, false
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == alb.ReadinessGate {
			return condition, true
		}
	}
	return v1.PodCondition{}, false
}

func setReadinessConditionStatus(pod *v1.Pod, condition v1.PodCondition) {
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

func readinessGateEnabled(pod *v1.Pod) bool {
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

func patchPodStatus(c client.Client, ctx context.Context, pod *v1.Pod) error {
	return c.Patch(ctx, pod, client.Apply)
}
