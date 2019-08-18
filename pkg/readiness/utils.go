package readiness

import (
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

func hasHostname(ingress *extensionsv1beta1.Ingress) bool {
	lbStatus := ingress.Status.LoadBalancer.Ingress
	if len(lbStatus) < 1 {
		return false
	}
	return true
}
