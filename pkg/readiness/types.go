package readiness

import (
	"github.com/nirnanaaa/kube-readiness/pkg/cloud"
	"k8s.io/apimachinery/pkg/types"
)

// IngressEndpoint contains the essential information for each pod in a endpoint group.
type IngressEndpoint struct {
	IP   string
	Port string
	Node string
}

type EndpointPodMap map[IngressEndpoint]types.NamespacedName

// IngressSet maps an ingress to endpoints
type IngressSet map[types.NamespacedName]IngressData

type IngressData struct {
	IngressEndpoints IngressEndpointSet
	LoadBalancer     LoadBalancerData
}

type LoadBalancerData struct {
	//TODO: Should we store the hostname or the name(arn) of the ALB here?
	Hostname  string
	Endpoints []cloud.EndpointGroup
}

func (i IngressSet) Ensure(name types.NamespacedName) IngressData {
	if _, ok := i[name]; !ok {
		i[name] = IngressData{IngressEndpointSet{}, LoadBalancerData{}}
	}
	return i[name]
}

func (i IngressSet) Remove(names ...types.NamespacedName) {
	for _, item := range names {
		delete(i, item)
	}
}

func (i IngressSet) FindByIP(ip string) IngressData {
	for _, item := range i {
		if item.IngressEndpoints.HasIp(ip) {
			return item
		}
	}
	return IngressData{IngressEndpointSet{}, LoadBalancerData{}}
}

// IngressEndpointSet maps pods to ingresses
type IngressEndpointSet map[IngressEndpoint]struct{}

func (i IngressEndpointSet) HasIp(ip string) bool {
	for isp := range i {
		if isp.IP == ip {
			return true
		}
	}
	return false
}

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
