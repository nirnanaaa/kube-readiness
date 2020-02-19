package readiness

// import (
// 	"context"
// 	"errors"

// 	corev1 "k8s.io/api/core/v1"
// 	apierrors "k8s.io/apimachinery/pkg/api/errors"
// 	"k8s.io/apimachinery/pkg/types"
// )

// var (
// 	PodNotReadyErr = errors.New("pod is not ready, yet")
// )

// func (r *Controller) podWorker() {
// 	for r.processNextPodWorkItem() {
// 	}
// }
// func (r *Controller) processNextPodWorkItem() bool {
// 	key, quit := r.podQueue.Get()
// 	if quit {
// 		return false
// 	}
// 	defer r.podQueue.Done(key)
// 	message := key.(types.NamespacedName)
// 	err := r.syncPodInternal(message)
// 	r.handlePodErr(err, message)

// 	return true
// }

// // handleErr handles errors from syncIngress
// func (r *Controller) handlePodErr(err error, msg types.NamespacedName) {
// 	if err == nil {
// 		r.podQueue.Forget(msg)
// 		return
// 	}
// 	r.Log.Info("received an error", "name", msg.String(), "error", err.Error())

// 	if r.podQueue.NumRequeues(msg) < maxRetries {
// 		r.podQueue.AddRateLimited(msg)
// 		return
// 	}
// 	r.podQueue.Forget(msg)
// }

// func (r *Controller) SyncPod(pod types.NamespacedName) {
// 	r.podQueue.Add(pod)
// }

// func (r *Controller) syncPodInternal(namespacedName types.NamespacedName) (err error) {
// 	ctx := context.Background()
// 	pod := &corev1.Pod{}
// 	if err := r.KubeSDK.Get(ctx, namespacedName, pod); err != nil {
// 		if apierrors.IsNotFound(err) {
// 			return nil
// 		}
// 		// Error reading the object - requeue the request.
// 		return err
// 	}
// 	log := r.Log.WithValues("pod", namespacedName.String())
// 	if !readinessGateEnabled(pod) {
// 		return nil
// 	}

// 	status, _ := readinessConditionStatus(pod)

// 	if status.Status == corev1.ConditionTrue {
// 		return nil
// 	}

// 	ingress, endpoint := r.IngressSet.FindByIP(pod.Status.PodIP)
// 	if len(ingress.IngressEndpoints) == 0 {
// 		return errors.New("pod does not belong to an ingress	")
// 	}

// 	healthy, err := r.CloudSDK.IsEndpointHealthy(ctx, ingress.LoadBalancer.Endpoints, pod.Status.PodIP, endpoint.Port)
// 	if err != nil {
// 		log.Error(err, "could not query target health")
// 		return err
// 	}

// 	if !healthy {
// 		log.Info("pod is not healthy, yet")
// 		status.Status = corev1.ConditionFalse
// 		if err := patchPodStatus(r.KubeSDK, ctx, pod, status); err != nil {
// 			return err
// 		}
// 		return PodNotReadyErr
// 	}

// 	status.Status = corev1.ConditionTrue
// 	return patchPodStatus(r.KubeSDK, ctx, pod, status)
// }
