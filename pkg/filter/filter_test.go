package filter

import (
	"io/ioutil"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/openshift-psap/special-resource-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func TestFilter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Filter Suite")
}

const (
	ownedLabel = "specialresource.openshift.io/owned"
	kind       = "SpecialResource"
)

var _ = Describe("IsSpecialResource", func() {
	cases := []struct {
		name    string
		obj     client.Object
		matcher types.GomegaMatcher
	}{
		{
			name: kind,
			obj: &v1beta1.SpecialResource{
				TypeMeta: metav1.TypeMeta{Kind: kind},
			},
			matcher: BeTrue(),
		},
		{
			name: "Pod owned by SRO",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{ownedLabel: "true"},
				},
			},
			matcher: BeFalse(),
		},
		{
			name: "valid selflink",
			obj: func() *unstructured.Unstructured {
				uo := &unstructured.Unstructured{}
				uo.SetSelfLink("/apis/sro.openshift.io/v1")

				return uo
			}(),
			matcher: BeTrue(),
		},
		{
			name: "selflink in Label",
			obj: func() *unstructured.Unstructured {
				uo := &unstructured.Unstructured{}
				uo.SetLabels(map[string]string{"some-label": "/apis/sro.openshift.io/v1"})

				return uo
			}(),
			matcher: BeTrue(),
		},
		{
			name:    "no selflink",
			obj:     &unstructured.Unstructured{},
			matcher: BeFalse(),
		},
	}

	entries := make([]TableEntry, 0, len(cases))

	for _, c := range cases {
		entries = append(entries, Entry(c.name, c.obj, c.matcher))
	}

	DescribeTable(
		"should return the correct value",
		func(obj client.Object, m types.GomegaMatcher) {
			f := filter{
				log:        zap.New(zap.WriteTo(ioutil.Discard)),
				kind:       kind,
				ownedLabel: ownedLabel,
			}
			Expect(f.isSpecialResource(obj)).To(m)
		},
		entries...)
})

var _ = Describe("Owned", func() {
	cases := []struct {
		name    string
		obj     client.Object
		matcher types.GomegaMatcher
	}{
		{
			name: "via ownerReferences",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{Kind: kind},
					},
				},
			},
			matcher: BeTrue(),
		},
		{
			name: "via labels",
			obj: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{ownedLabel: "whatever"},
				},
			},
			matcher: BeTrue(),
		},
		{
			name:    "not owned",
			obj:     &corev1.Pod{},
			matcher: BeFalse(),
		},
	}

	entries := make([]TableEntry, 0, len(cases))

	for _, c := range cases {
		entries = append(entries, Entry(c.name, c.obj, c.matcher))
	}

	DescribeTable(
		"should return the expected value",
		func(obj client.Object, m types.GomegaMatcher) {
			f := filter{
				log:        zap.New(zap.WriteTo(ioutil.Discard)),
				kind:       kind,
				ownedLabel: ownedLabel,
			}
			Expect(f.owned(obj)).To(m)
		},
		entries...,
	)
})

var _ = Describe("Predicate", func() {
	var f filter

	BeforeEach(func() {
		f = filter{
			log:        zap.New(zap.WriteTo(ioutil.Discard)),
			kind:       kind,
			ownedLabel: ownedLabel,
		}
	})

	Context("CreateFunc", func() {
		cases := []struct {
			name       string
			obj        client.Object
			retMatcher types.GomegaMatcher
		}{
			{
				name:       "special resource",
				obj:        &v1beta1.SpecialResource{},
				retMatcher: BeTrue(),
			},
			{
				name: "owned",
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{Kind: kind},
						},
					},
				},
				retMatcher: BeTrue(),
			},
			{
				name:       "random pod",
				obj:        &corev1.Pod{},
				retMatcher: BeFalse(),
			},
		}

		entries := make([]TableEntry, 0, len(cases))

		for _, c := range cases {
			entries = append(entries, Entry(c.name, c.obj, c.retMatcher))
		}

		DescribeTable(
			"should work as expected",
			func(obj client.Object, m types.GomegaMatcher) {
				ret := f.GetPredicates().Create(event.CreateEvent{Object: obj})

				Expect(ret).To(m)
				Expect(f.GetMode()).To(Equal("CREATE"))
			},
			entries...,
		)
	})

	Context("UpdateFunc", func() {
		It("should work as expected", func() {
			Skip("Testing this function requires injecting a fake ClientSet")
		})
	})

	Context("DeleteFunc", func() {
		cases := []struct {
			name       string
			obj        client.Object
			retMatcher types.GomegaMatcher
		}{
			{
				name:       "special resource",
				obj:        &v1beta1.SpecialResource{},
				retMatcher: BeTrue(),
			},
			// TODO(qbarrand) testing this function requires injecting a fake ClientSet
			//{ name: "owned" },
			{
				name:       "random pod",
				obj:        &corev1.Pod{},
				retMatcher: BeFalse(),
			},
		}

		entries := make([]TableEntry, 0, len(cases))

		for _, c := range cases {
			entries = append(entries, Entry(c.name, c.obj, c.retMatcher))
		}

		DescribeTable(
			"should work as expected",
			func(obj client.Object, m types.GomegaMatcher) {
				ret := f.GetPredicates().Delete(event.DeleteEvent{Object: obj})

				Expect(ret).To(m)
				Expect(f.GetMode()).To(Equal("DELETE"))
			},
			entries...,
		)
	})

	Context("GenericFunc", func() {
		cases := []struct {
			name       string
			obj        client.Object
			retMatcher types.GomegaMatcher
		}{
			{
				name:       "special resource",
				obj:        &v1beta1.SpecialResource{},
				retMatcher: BeTrue(),
			},
			{
				name: "owned",
				obj: &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{Kind: kind},
						},
					},
				},
				retMatcher: BeTrue(),
			},
			{
				name:       "random pod",
				obj:        &corev1.Pod{},
				retMatcher: BeFalse(),
			},
		}

		entries := make([]TableEntry, 0, len(cases))

		for _, c := range cases {
			entries = append(entries, Entry(c.name, c.obj, c.retMatcher))
		}

		DescribeTable(
			"should return the correct value",
			func(obj client.Object, m types.GomegaMatcher) {
				ret := f.GetPredicates().Generic(event.GenericEvent{Object: obj})

				Expect(ret).To(m)
				Expect(f.GetMode()).To(Equal("GENERIC"))
			},
			entries...,
		)
	})
})
