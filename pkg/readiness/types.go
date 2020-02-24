package readiness

import (
	"errors"

	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"k8s.io/apimachinery/pkg/types"
)

// IngressEndpoint contains the essential information for each pod in a endpoint group.
type IngressEndpoint struct {
	IP   string
	Port int32
}

type EndpointPodMap map[IngressEndpoint]types.NamespacedName

// ServiceInfoMap stores a service and its ingressdata
type ServiceInfoMap map[types.NamespacedName]IngressInfo

type IngressInfo struct {
	Name      string
	Endpoints []*cloud.EndpointGroup
	Pods      []types.NamespacedName
}

func (i ServiceInfoMap) GetServiceInfoForPod(name types.NamespacedName) (*IngressInfo, error) {
	for _, item := range i {
		for _, podName := range item.Pods {
			if podName == name {
				return &item, nil
			}
		}
	}
	return nil, errors.New("could not find serviceinfo for pod name")
}

func (i ServiceInfoMap) Add(name types.NamespacedName, info IngressInfo) IngressInfo {
	i[name] = info
	return i[name]
}

func (i ServiceInfoMap) Remove(names ...types.NamespacedName) {
	for _, item := range names {
		delete(i, item)
	}
}
