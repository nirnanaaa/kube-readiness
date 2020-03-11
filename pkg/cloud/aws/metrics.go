package aws

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	successfulApiRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name:      "successful_api_requests",
			Namespace: "aws",
			Help:      "Number of successful aws api requests",
		},
	)
	throttledApiRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name:      "throttled_api_requests",
			Namespace: "aws",
			Help:      "Number of throttled aws api requests",
		},
	)
	failedApiRequests = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name:      "failed_api_requests",
			Namespace: "aws",
			Help:      "Number of failed aws api requests",
		},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(successfulApiRequests, throttledApiRequests, failedApiRequests)
}
