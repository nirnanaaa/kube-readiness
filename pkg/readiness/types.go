package readiness

import (
	"k8s.io/apimachinery/pkg/types"
)

// IngressEndpoint contains the essential information for each network endpoint in a NEG
type IngressEndpoint struct {
	IP   string
	Port string
	Node string
}

type EndpointPodMap map[IngressEndpoint]types.NamespacedName

// IngressSet maps an ingress to endpoints
type IngressSet map[types.NamespacedName]*IngressEndpointSet

func (i IngressSet) Ensure(name types.NamespacedName) *IngressEndpointSet {
	if _, ok := i[name]; !ok {
		i[name] = &IngressEndpointSet{}
	}
	return i[name]
}

func (i IngressSet) Remove(names ...types.NamespacedName) {
	for _, item := range names {
		delete(i, item)
	}
}

// IngressEndpointSet maps pods to ingresses
type IngressEndpointSet map[IngressEndpoint]struct{}

// Insert adds items to the set.
func (i IngressEndpointSet) Insert(items ...IngressEndpoint) {
	for _, item := range items {
		i[item] = struct{}{}
	}
}

// Has returns true if and only if item is contained in the set.
func (i IngressEndpointSet) Has(item IngressEndpoint) bool {
	_, contained := i[item]
	return contained
}

// Delete removes all items from the set.
func (i IngressEndpointSet) Delete(items ...IngressEndpoint) {
	for _, item := range items {
		delete(i, item)
	}
}

// Len returns the size of the set.
func (i IngressEndpointSet) Len() int {
	return len(i)
}