package readiness

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

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

func (r *Controller) SyncPod(pod types.NamespacedName) {
	r.podQueue.Add(pod)
}

func (r *Controller) syncPodInternal(namespacedName types.NamespacedName) (err error) {
	log := r.Log.WithValues("trigger", "scheduled")
	ctx := context.Background()
	pod := &corev1.Pod{}
	if err := r.KubeSDK.Get(ctx, namespacedName, pod); err != nil {
		if apierrors.IsNotFound(err) {
			// TODO: remove here from ingress map

			log.Info("pod not found, skipping")
			return nil
		}
		// Error reading the object - requeue the request.
		return err
	}

	//If this is a pod beeing deleted remove it from ALB
	if pod.DeletionTimestamp != nil {
		log.Info("received an Pod deletion", "name", pod.Name, "namespace", pod.Namespace)
		ingress, endpoint := r.IngressSet.FindByIP(pod.Status.PodIP)
		if len(ingress.IngressEndpoints) == 0 {
			log.Info("pod does not have an ingress")
			return nil
		}
		err := r.CloudSDK.RemoveEndpoint(ctx, ingress.LoadBalancer.Endpoints, pod.Status.PodIP, endpoint.Port)
		if err != nil {
			log.Error(err, "could not remove endpoint")
			return err
		}
		log.Info("pod Endpoint removed from AWS")
		return nil
	}

	if !readinessGateEnabled(pod) {
		log.Info("pod does not have readiness gates enabled.", "name", pod.Name, "namespace", pod.Namespace)
		return nil
	}
	status, _ := readinessConditionStatus(pod)

	// TODO: remove this as soon as we handle some different status than true
	if status.Status == corev1.ConditionTrue {
		log.Info("pod is already ready. skipping check", "name", pod.Name, "namespace", pod.Namespace)
		return nil
	}
	ingress, endpoint := r.IngressSet.FindByIP(pod.Status.PodIP)
	if len(ingress.IngressEndpoints) == 0 {
		return errors.New("pod does not have ingress yet")
	}

	healthy, err := r.CloudSDK.IsEndpointHealthy(ctx, ingress.LoadBalancer.Endpoints, pod.Status.PodIP, endpoint.Port)
	if err != nil {
		log.Error(err, "something was wrong when gathering target health")
		return err
	}
	if healthy {
		status.Status = corev1.ConditionTrue
	}
	if err := patchPodStatus(r.KubeSDK, ctx, pod, status); err != nil {
		return err
	}
	if healthy {
		return nil
	}
	return errors.New("pod not healthy yet")
}
