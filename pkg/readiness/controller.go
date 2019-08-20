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
	return &Controller{
		ingressQueue:   workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		podQueue:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
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
func (r *Controller) podWorker() {
	for r.processNextPodWorkItem() {
	}
}
func (r *Controller) processNextPodWorkItem() bool {
	key, quit := r.podQueue.Get()
	if quit {
		return false
	}
	defer r.podQueue.Done(key)
	message := key.(types.NamespacedName)
	err := r.syncPodInternal(message)
	r.handlePodErr(err, message)

	return true
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
func (r *Controller) handlePodErr(err error, msg types.NamespacedName) {
	if err == nil {
		r.podQueue.Forget(msg)
		return
	}
	r.Log.Info("received an error", "name", msg.String(), "error", err.Error())

	if r.podQueue.NumRequeues(msg) < maxRetries {
		r.podQueue.AddRateLimited(msg)
		return
	}
	r.podQueue.Forget(msg)
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

func (r *Controller) SyncPod(pod types.NamespacedName) {
	r.podQueue.Add(pod)
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
					return errors.New("retry service was not available")
				}
				// Error reading the object - requeue the request.
				return err
			}
			//Find the endpoints for the service
			eps := &corev1.Endpoints{}
			if err := r.KubeSDK.Get(ctx, svcKey, eps); err != nil {
				log.Error(err, "something wrong happend when fetching Endpoint")
				return err
			}
			for _, sub := range eps.Subsets {
				//TODO there are multiple ports which one to use?
				for _, add := range sub.Addresses {
					ingressData.IngressEndpoints.Insert(IngressEndpoint{
						IP: add.IP,
					})
				}
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
	var tmp []cloud.EndpointGroup
	for _, v := range endpoints {
		tmp = append(tmp, *v)
	}
	ingressData.LoadBalancer.Endpoints = tmp
	ingressData.LoadBalancer.Hostname = hostname

	r.IngressSet[namespacedName] = ingressData

	//TODO: how do we handle host mode vs ip mode

	log.Info(fmt.Sprintf("received Ingress [%s] with hostname [%s], containing following LoadBalancer endpoints and Service endpoits", namespacedName.String(), hostname))
	for _, endpoint := range ingressData.LoadBalancer.Endpoints {
		log.Info(fmt.Sprintf("LoadBalancer endpoint name [%s]", endpoint.Name))
	}
	for endpoint := range ingressData.IngressEndpoints {
		log.Info(fmt.Sprintf("Ingress endpoint name [%s]", endpoint.IP))
	}

	fmt.Println("zzzzzzzzzzzzzzzzzzz")
	fmt.Println(r.IngressSet)

	return
}

func (r *Controller) syncPodInternal(namespacedName types.NamespacedName) (err error) {
	log := r.Log.WithValues("trigger", "scheduled")
	ctx := context.Background()
	log.Info("received pod update")
	pod := &corev1.Pod{}
	if err := r.KubeSDK.Get(ctx, namespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("pod not found, skipping")
			return nil
		}
		// Error reading the object - requeue the request.
		return err
	}
	ingress := r.IngressSet.FindByIP(pod.Status.PodIP)
	if len(ingress.IngressEndpoints) == 0 {
		return errors.New("pod does not have ingress yet")
	}
	status, _ := readinessConditionStatus(pod)

	//TODO: We need to pass the port as well (store it as well in IngressSet)
	healthy, err := r.CloudSDK.IsEndpointHealthy(ctx, ingress.LoadBalancer.Endpoints, pod.Status.PodIP)
	if err != nil {
		log.Error(err, "something was wrong when gathering target health")
		return err
	}
	if healthy {
		status.Status = corev1.ConditionTrue
		setReadinessConditionStatus(pod, status)
		if err := patchPodStatus(r.KubeSDK, ctx, pod); err != nil {
			return err
		}
		//TODO: on healthy set Annotation to ready
		return nil
	}
	return errors.New("pod not healthy yet")
}
