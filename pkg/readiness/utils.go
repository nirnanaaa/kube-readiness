package readiness

import (
	"errors"

	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
)

func extractHostname(ingress *extensionsv1beta1.Ingress) (string, error) {
	lbStatus := ingress.Status.LoadBalancer.Ingress
	if len(lbStatus) < 1 {
		return "", errors.New("ingress not ready, yet. requeue")
	}
	//TODO: ingress.Status.LoadBalancer.Ingress is a list, how many can we have? which one to use?
	return ingress.Status.LoadBalancer.Ingress[0].Hostname, nil
}
