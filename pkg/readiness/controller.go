package readiness

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
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
	CloudSDK       cloud.SDK
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
	log.Info("received ingress update")
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

	ingressData := r.IngressSet.Ensure(namespacedName)

	hostname, err := extractHostname(ingress)
	if err != nil {
		return errors.New("ingress not ready, yet. requeue")
	}
	ingressData.LoadBalancer.Hostname = hostname

	log.Info(fmt.Sprintf("all ingress information we store: %v", ingressData))

	//TODO: Find endpoints and store them in the SET

	log.Info("ensuring ingress is up to date with aws api")
	//TODO: Find ARN, find TargetGroups for ALB using ARN, store them in the SET
	_, err = r.CloudSDK.GetEndpointGroupsByHostname(context.Background(), hostname)
	if err != nil {
		return errors.New("error fetching info from aws sdk")
	}
	return
}
