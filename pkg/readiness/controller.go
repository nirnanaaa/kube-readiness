package readiness

import (
	"time"

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
)

type Controller struct {
	Log            logr.Logger
	EndpointPodMap EndpointPodMap
	IngressSet     IngressSet
	queue          workqueue.RateLimitingInterface
}

func NewController() *Controller {
	return &Controller{
		queue:          workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		EndpointPodMap: make(EndpointPodMap),
		IngressSet:     make(IngressSet),
	}
}

func (r *Controller) Run(stopCh <-chan struct{}) {
	defer r.queue.ShutDown()
	go wait.Until(r.worker, time.Second, stopCh)
	<-stopCh
}

func (r *Controller) worker() {
	for r.processNextWorkItem() {
	}
}

func (r *Controller) processNextWorkItem() bool {
	key, quit := r.queue.Get()
	if quit {
		return false
	}
	defer r.queue.Done(key)
	r.syncIngressInternal(key.(types.NamespacedName))
	return true
}

func (r *Controller) SyncIngress(ing types.NamespacedName) {
	r.queue.AddRateLimited(ing)
}

// query AWS for that ingress with namespacedName %s, processing is done asynchronously
// after it new into should be added to r.IngressSet / r.EndpointPodMap
func (r *Controller) syncIngressInternal(namespacedName types.NamespacedName) {
	log := r.Log.WithValues("trigger", "scheduled")
	log.Info("received for ingress")
	_ = r.IngressSet.Ensure(namespacedName)
}
