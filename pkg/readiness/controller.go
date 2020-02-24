package readiness

import (
	"context"
	"errors"
	"time"

	"github.com/go-logr/logr"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	BackendNotFoundErr = errors.New("no backend found")
	NoMatchingPortErr  = errors.New("no port matched")
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
	// IngressSet     IngressSet
	ingressQueue workqueue.RateLimitingInterface
	// podQueue       workqueue.RateLimitingInterface
	CloudSDK cloud.SDK
	KubeSDK  client.Client
}

func NewController(kube client.Client, endpointPodMap EndpointPodMap) *Controller {
	//TODO: When things fail NewRateLimitingQueue resends rather quickly, what do we do about that?
	//Potentialy if it sends to fast and alb-ingress-controller is to slow it might miss the info of hostname
	slowRateLimiter := workqueue.NewItemExponentialFailureRateLimiter(1*time.Second, 1000*time.Second)
	return &Controller{
		ingressQueue: workqueue.NewRateLimitingQueue(slowRateLimiter),
		// podQueue:       workqueue.NewRateLimitingQueue(slowRateLimiter),
		EndpointPodMap: endpointPodMap,
		// IngressSet:     ingressSet,
		KubeSDK: kube,
	}
}

func (r *Controller) Run(stopCh <-chan struct{}) {
	// defer r.podQueue.ShutDown()
	defer r.ingressQueue.ShutDown()
	go wait.Until(r.ingressWorker, time.Second, stopCh)
	// go wait.Until(r.podWorker, time.Second, stopCh)
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
	// ctx := context.Background()
	// ingress := &extensionsv1beta1.Ingress{}
	// if err := r.KubeSDK.Get(ctx, namespacedName, ingress); err != nil {
	// 	if apierrors.IsNotFound(err) {
	// 		return nil
	// 	}
	// 	// Error reading the object - requeue the request.
	// 	return err
	// }
	// log := r.Log.WithValues("ingress", namespacedName.String())

	// ingressData := r.IngressSet.Ensure(namespacedName)
	// log.Info("extracing hostname from ingress")

	// hostname, err := extractHostname(ingress)
	// if err != nil {
	// 	return err
	// }
	// log.WithValues("hostname", hostname).Info("extracted hostname from ingress")

	// //Find all services for Ingress
	// if len(ingress.Spec.Rules) < 1 && ingress.Spec.Backend == nil {
	// 	log.Info("no rules found in backend spec")
	// 	return nil
	// }
	// serviceName, servicePort, err := r.collectIngressBackendService(ctx, ingress)
	// if err != nil {
	// 	if err == BackendNotFoundErr {
	// 		log.Info("ingress doesn't have a backend")
	// 		return nil
	// 	}
	// 	return err
	// }

	// //Find the endpoints for the service
	// serviceEndpoints := &corev1.Endpoints{}
	// svcKey := types.NamespacedName{
	// 	Namespace: namespacedName.Namespace,
	// 	Name:      serviceName,
	// }
	// if err := r.KubeSDK.Get(ctx, svcKey, serviceEndpoints); err != nil {
	// 	return err
	// }
	// for _, endpointSubset := range serviceEndpoints.Subsets {
	// 	portIdx, err := getPortFromEndpointIdxPortMap(servicePort, &endpointSubset)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	// for _, endpointAddress := range endpointSubset.Addresses {
	// 	// 	ingressData.IngressEndpoints.Insert(IngressEndpoint{
	// 	// 		IP:   endpointAddress.IP,
	// 	// 		Port: endpointSubset.Ports[portIdx].Port,
	// 	// 	})
	// 	// }
	// 	// for _, endpointAddress := range endpointSubset.NotReadyAddresses {
	// 	// 	ingressData.IngressEndpoints.Insert(IngressEndpoint{
	// 	// 		IP:   endpointAddress.IP,
	// 	// 		Port: endpointSubset.Ports[portIdx].Port,
	// 	// 	})
	// 	// }
	// }

	// endpointsByHostname, err := r.CloudSDK.GetEndpointGroupsByHostname(context.Background(), hostname)
	// if err != nil {
	// 	return err
	// }
	// var endpointGroup []cloud.EndpointGroup
	// for _, endpoint := range endpointsByHostname {
	// 	endpointGroup = append(endpointGroup, *endpoint)
	// }
	// ingressData.LoadBalancer.Endpoints = endpointGroup
	// ingressData.LoadBalancer.Hostname = hostname

	// r.IngressSet[namespacedName] = ingressData
	return
}

func (r *Controller) collectIngressBackendService(ctx context.Context, ingress *extensionsv1beta1.Ingress) (serviceName string, port intstr.IntOrString, err error) {
	if ingress.Spec.Backend != nil {
		return ingress.Spec.Backend.ServiceName, ingress.Spec.Backend.ServicePort, nil
	}
	for _, rule := range ingress.Spec.Rules {
		if len(rule.IngressRuleValue.HTTP.Paths) < 1 {
			continue
		}
		for _, p := range rule.IngressRuleValue.HTTP.Paths {
			service := &corev1.Service{}
			svcKey := types.NamespacedName{
				Namespace: ingress.Namespace,
				Name:      p.Backend.ServiceName,
			}
			if err := r.KubeSDK.Get(ctx, svcKey, service); err != nil {
				continue
			}
			return p.Backend.ServiceName, p.Backend.ServicePort, nil
		}
	}
	return "", intstr.FromString(""), BackendNotFoundErr
}

func getPortFromEndpointIdxPortMap(servicePort intstr.IntOrString, endpointSubset *corev1.EndpointSubset) (portIdx int, err error) {
	for idx, port := range endpointSubset.Ports {
		if port.Name == servicePort.String() {
			return idx, nil
		}
		if port.Port == int32(servicePort.IntValue()) {
			return idx, nil
		}
	}
	return 0, NoMatchingPortErr
}
