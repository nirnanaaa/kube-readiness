package readiness

import (
	"context"
	"errors"
	"fmt"
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

type Controller struct {
	Log            logr.Logger
	EndpointPodMap EndpointPodMap
	IngressSet     IngressSet
	queue          workqueue.RateLimitingInterface
	CloudSDK       cloud.SDK
	KubeSDK        client.Client
}

func NewController(kube client.Client) *Controller {
	//TODO: When things fail NewRateLimitingQueue resends rather quickly, what do we do about that?
	//Potentialy if it sends to fast and alb-ingress-controller is to slow it might miss the info of hostname
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

	//Find all services for Ingress
	if len(ingress.Spec.Rules) < 1 {
		log.Info("Ingress has no rules, therefore no services")
		return nil
	}
	for _, rule := range ingress.Spec.Rules {
		if len(rule.IngressRuleValue.HTTP.Paths) < 1 {
			log.Info("Ingress Spec has no Paths, therefore no services")
			return nil
		}
		for _, p := range rule.IngressRuleValue.HTTP.Paths {
			//TODO: Is the assumption correct that both Ingress and Service are in the same namespace?
			service := &corev1.Service{}
			svcKey := types.NamespacedName{
				Namespace: namespacedName.Namespace,
				Name:      p.Backend.ServiceName,
			}
			if err := r.KubeSDK.Get(ctx, svcKey, service); err != nil {
				if apierrors.IsNotFound(err) {
					log.Info("could not find service: " + svcKey.String())
					return errors.New("retry service whas not available")
				}
				// Error reading the object - requeue the request.
				return err
			}
			//Find the endpoints for the service
			eps := &corev1.Endpoints{}
			if err := r.KubeSDK.Get(ctx, svcKey, eps); err != nil {
				log.Error(err, "something wrong happen when fetching Endpoint")
				return err
			}
			for _, sub := range eps.Subsets {
				//TODO there are multiple ports which one to use?
				//Only check the NotReadyAddresses only
				for _, add := range sub.NotReadyAddresses {
					ingressData.IngressEndpoints.Insert(IngressEndpoint{
						IP: add.IP,
					})
				}
			}

		}
	}

	log.Info("ensuring ingress is up to date with aws api")
	endpoints, err := r.CloudSDK.GetEndpointGroupsByHostname(context.Background(), hostname)
	if err != nil {
		log.Error(err, "error fetching info from aws sdk")
		return errors.New("error fetching info from aws sdk")
	}
	ingressData.LoadBalancer.Endpoints = endpoints

	//TODO: how do we handle host mode vs ip mode

	log.Info(fmt.Sprintf("received Ingress [%s] with hostname [%s], containing following LoadBalancer endpoints and Service endpoits", namespacedName.String(), hostname))
	for _, endpoint := range ingressData.LoadBalancer.Endpoints {
		log.Info(fmt.Sprintf("LoadBalancer endpoint name [%s]", endpoint.Name))
	}
	for endpoint := range *ingressData.IngressEndpoints {
		log.Info(fmt.Sprintf("Ingress endpoint name [%s]", endpoint.IP))
	}

	return
}
