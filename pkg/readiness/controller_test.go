package readiness

import (
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var _ = Describe("Readiness Types", func() {
	const timeout = time.Second * 5
	const interval = time.Second * 1
	var controller *Controller
	var stopCh chan struct{}
	BeforeEach(func() {
		controller = NewController(k8sClient)
		controller.Log = ctrl.Log.WithName("controllers").WithName("Readiness")
		stopCh = make(chan struct{})
		go controller.Run(stopCh)
	})

	AfterEach(func() {
		close(stopCh)
	})

	Context("Controller", func() {
		It("should add an ingress to check", func() {
			name := types.NamespacedName{
				Namespace: "test-system",
				Name:      "test",
			}
			controller.SyncIngress(name)
			Eventually(func() *IngressEndpointSet {
				return controller.IngressSet[name]
			}, timeout, interval).ShouldNot(BeNil())
		})
	})
})
