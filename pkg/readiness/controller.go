package readiness

import (
	"time"

	"github.com/go-logr/logr"
)

type Controller struct {
	Log            logr.Logger
	EndpointPodMap EndpointPodMap
	IngressSet     IngressSet
}

func (r *Controller) RunLoop(duration time.Duration, stopCh chan bool) (err error) {
	ticker := time.NewTicker(duration)
	logger := r.Log.WithValues("trigger", "scheduled")

	for {
		select {
		case <-ticker.C:
			// iterate over all ingresses
			// get endpoints in TG
			// build up endpoint map (EndpointPodMap)
			for albARN, endpoints := range r.IngressSet {
				logger.Info("debug", "ingress", albARN, "endpoints", endpoints.Len())
			}
		case <-stopCh:
			return
		}
	}
}
