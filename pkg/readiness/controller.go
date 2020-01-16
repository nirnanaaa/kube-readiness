package readiness

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	corev1 "k8s.io/api/core/v1"
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

type Message struct {
	Type           string
	NamespacedName types.NamespacedName
}

type Controller struct {
	Log            logr.Logger
	EndpointPodMap EndpointPodMap
	IngressSet     IngressSet
	ingressQueue   workqueue.RateLimitingInterface
	podQueue       workqueue.RateLimitingInterface
	CloudSDK       cloud.SDK
	KubeSDK        client.Client
}

func NewController(kube client.Client) *Controller {
	//TODO: When things fail NewRateLimitingQueue resends rather quickly, what do we do about that?
	//Potentialy if it sends to fast and alb-ingress-controller is to slow it might miss the info of hostname
	slowRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 1000*time.Second)
	return &Controller{
		ingressQueue:   workqueue.NewRateLimitingQueue(slowRateLimiter),
		podQueue:       workqueue.NewRateLimitingQueue(slowRateLimiter),
		EndpointPodMap: make(EndpointPodMap),
		IngressSet:     make(IngressSet),
		KubeSDK:        kube,
	}
}

func (r *Controller) Run(stopCh <-chan struct{}) {
	defer r.podQueue.ShutDown()
	defer r.ingressQueue.ShutDown()
	go wait.Until(r.ingressWorker, time.Second, stopCh)
	go wait.Until(r.podWorker, time.Second, stopCh)
	<-stopCh
}

func (r *Controller) ingressWorker() {
	for r.processNextIngressWorkItem() {
	}
}

func (r *Controller) processNextIngressWorkItem() bool {
	key, quit := r.ingressQueue.Get()
	if quit {
		return false
	}
	defer r.ingressQueue.Done(key)
	message := key.(types.NamespacedName)
	err := r.syncIngressInternal(message)
	r.handleIngressErr(err, message)

	return true
}

// handleErr handles errors from syncIngress
func (r *Controller) handleIngressErr(err error, msg types.NamespacedName) {
	if err == nil {
		r.ingressQueue.Forget(msg)
		return
	}
	r.Log.Info("received an error", "name", msg.String(), "error", err.Error())

	if r.ingressQueue.NumRequeues(msg) < maxRetries {
		r.ingressQueue.AddRateLimited(msg)
		return
	}
	r.ingressQueue.Forget(msg)
}

func (r *Controller) SyncIngress(ing types.NamespacedName) {
	r.ingressQueue.Add(ing)
}

// query AWS for that ingress with namespacedName %s, processing is done asynchronously
// after it new into should be added to r.IngressSet / r.EndpointPodMap
func (r *Controller) syncIngressInternal(namespacedName types.NamespacedName) (err error) {
	log := r.Log.WithValues("trigger", "scheduled")
	ctx := context.Background()
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

	//Find all services for Ingress
	if len(ingress.Spec.Rules) < 1 {
		return nil
	}
	for _, rule := range ingress.Spec.Rules {
		if len(rule.IngressRuleValue.HTTP.Paths) < 1 {
			log.Info("Ingress Spec has no Paths, therefore no services")
			continue
		}
		for _, p := range rule.IngressRuleValue.HTTP.Paths {
			service := &corev1.Service{}
			svcKey := types.NamespacedName{
				Namespace: namespacedName.Namespace,
				Name:      p.Backend.ServiceName,
			}
			if err := r.KubeSDK.Get(ctx, svcKey, service); err != nil {
				if apierrors.IsNotFound(err) {
					return errors.New("service could not be found")
				}
				return err
			}

			//Find the endpoints for the service
			eps := &corev1.Endpoints{}
			if err := r.KubeSDK.Get(ctx, svcKey, eps); err != nil {
				return err
			}
			for _, sub := range eps.Subsets {
				//TODO there are multiple ports which one to use?
				for _, add := range sub.Addresses {
					ingressData.IngressEndpoints.Insert(IngressEndpoint{
						IP:   add.IP,
						Port: sub.Ports[0].Port,
					})
				}
				for _, add := range sub.NotReadyAddresses {
					ingressData.IngressEndpoints.Insert(IngressEndpoint{
						IP:   add.IP,
						Port: sub.Ports[0].Port,
					})
				}
			}

		}
	}

	endpoints, err := r.CloudSDK.GetEndpointGroupsByHostname(context.Background(), hostname)
	if err != nil {
		return errors.New("error fetching info from aws sdk")
	}
	var tmp []cloud.EndpointGroup
	for _, v := range endpoints {
		tmp = append(tmp, *v)
	}
	ingressData.LoadBalancer.Endpoints = tmp
	ingressData.LoadBalancer.Hostname = hostname

	r.IngressSet[namespacedName] = ingressData
	return
}
