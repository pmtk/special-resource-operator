package metrics

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	dto "github.com/prometheus/client_model/go"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

func findMetric(src []*dto.MetricFamily, query string) *dto.MetricFamily {
	for _, s := range src {
		if s.Name != nil && *s.Name == query {
			return s
		}
	}
	return nil
}

var _ = Describe("Metrics", func() {
	value := 12
	sr := "simple-kmod"
	state := "templates/0000-buildconfig.yaml"
	kind := "BuildConfig"
	name := "simple-kmod-driver-build"
	namespace := "openshift-special-resource-operator"

	m := New()
	m.SetSpecialResourcesCreated(value)
	m.SetCompletedState(sr, state, value)
	m.SetCompletedKind(sr, kind, name, namespace, value)

	It("correctly passes calls to the collectors", func() {
		expected := []struct {
			query string
			value int
		}{
			{createdSpecialResourcesQuery, value},
			{completedStatesQuery, value},
			{completedKindQuery, value},
		}

		data, err := metrics.Registry.Gather()
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(HaveLen(len(expected)))

		for _, e := range expected {
			m := findMetric(data, e.query)
			Expect(m).ToNot(BeNil(), "metric for %s could not be found", e.query)
			Expect(m.Metric).To(HaveLen(1))
			Expect(m.Metric[0].Gauge).ToNot(BeNil())
			Expect(m.Metric[0].Gauge.Value).ToNot(BeNil())
			Expect(*m.Metric[0].Gauge.Value).To(BeEquivalentTo(value))
		}
	})
})
