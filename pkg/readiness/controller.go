package readiness

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud/aws"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	maxRetries = 15
)

type Controller struct {
	Log            logr.Logger
	EndpointPodMap EndpointPodMap
	IngressSet     IngressSet
	queue          workqueue.RateLimitingInterface
	CloudSDK       aws.SDK
	KubeSDK        client.Client
}

func NewController(kube client.Client) *Controller {
	return &Controller{
		queue:          workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		EndpointPodMap: make(EndpointPodMap),
		IngressSet:     make(IngressSet),
		KubeSDK:        kube,
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
	namespacedKey := key.(types.NamespacedName)
	err := r.syncIngressInternal(namespacedKey)
	r.handleErr(err, namespacedKey)
	return true
}

// handleErr handles errors from syncIngress
func (r *Controller) handleErr(err error, key types.NamespacedName) {
	if err == nil {
		r.queue.Forget(key)
		return
	}
	r.Log.Info("received an error for ingress", "name", key.String(), "error", err.Error())

	if r.queue.NumRequeues(key) < maxRetries {
		r.queue.AddRateLimited(key)
		return
	}
	r.queue.Forget(key)
}

func (r *Controller) SyncIngress(ing types.NamespacedName) {
	r.queue.Add(ing)
}

// query AWS for that ingress with namespacedName %s, processing is done asynchronously
// after it new into should be added to r.IngressSet / r.EndpointPodMap
func (r *Controller) syncIngressInternal(namespacedName types.NamespacedName) (err error) {
	log := r.Log.WithValues("trigger", "scheduled")
	ctx := context.Background()
	log.Info("received for ingress")
	ingress := &extensionsv1beta1.Ingress{}
	if err := r.KubeSDK.Get(ctx, namespacedName, ingress); err != nil {
		if apierrors.IsNotFound(err) {
			r.IngressSet.Remove(namespacedName)
			log.Info("removing not found ingress")
			return nil
		}
		// Error reading the object - requeue the request.
		return err
	}
	if !hasHostname(ingress) {
		return errors.New("ingress not ready, yet. requeue")
	}
	log.Info("ensuring ingress is up to date with aws api")
	// r.CloudSDK.FetchLoadBalancer()
	_ = r.IngressSet.Ensure(namespacedName)
	return
}
