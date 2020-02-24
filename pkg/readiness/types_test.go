package readiness

// import (
// 	. "github.com/onsi/ginkgo"
// 	. "github.com/onsi/gomega"
// 	"k8s.io/apimachinery/pkg/types"
// )

// var _ = Describe("Readiness Types", func() {
// 	Context("Ingress map", func() {
// 		It("Should build and get an ingress map correctly", func() {
// 			By("creating an ingress map")
// 			igMap := make(IngressSet)

// 			ingress := types.NamespacedName{
// 				Name:      "abc",
// 				Namespace: "some-system",
// 			}
// 			endpointMap := igMap.Ensure(ingress)
// 			Expect(endpointMap.IngressEndpoints.Len()).Should(Equal(0))
// 			By("adding an ingress")
// 			ep := IngressEndpoint{
// 				IP:   "10.10.0.1",
// 				Port: 80,
// 				Node: "10.10.0.0",
// 			}
// 			endpointMap.IngressEndpoints.Insert(ep)
// 			Expect(endpointMap.IngressEndpoints.Len()).Should(Equal(1))
// 			By("removing the ingress")
// 			endpointMap.IngressEndpoints.Delete(ep)
// 			Expect(endpointMap.IngressEndpoints.Len()).Should(Equal(0))
// 		})
// 	})
// })
