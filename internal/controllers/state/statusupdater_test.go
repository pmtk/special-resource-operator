package state_test

import (
	"context"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-psap/special-resource-operator/api/v1beta1"
	"github.com/openshift-psap/special-resource-operator/internal/controllers/state"
	"github.com/openshift-psap/special-resource-operator/pkg/clients"
	v1 "k8s.io/api/apps/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("StatusUpdater", func() {
	var mockKubeClient *clients.MockClientsInterface

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		mockKubeClient = clients.NewMockClientsInterface(ctrl)
	})

	Describe("UpdateWithState", func() {
		const (
			srName      = "sr-name"
			srNamespace = "sr-namespace"
		)

		It("should do nothing if the SpecialResource could not be found", func() {
			sr := &v1beta1.SpecialResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      srName,
					Namespace: srNamespace,
				},
			}

			mockKubeClient.
				EXPECT().
				Get(context.TODO(), types.NamespacedName{Name: srName, Namespace: srNamespace}, &v1beta1.SpecialResource{}).
				Return(k8serrors.NewNotFound(v1.Resource("specialresources"), srName))

			state.NewStatusUpdater(mockKubeClient).UpdateWithState(context.TODO(), sr, "test")
		})

		It("should update the SpecialResource in Kubernetes if it exists", func() {
			const newState = "test"

			sr := &v1beta1.SpecialResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      srName,
					Namespace: srNamespace,
				},
			}

			srWithState := sr.DeepCopy()
			srWithState.Status.State = newState

			gomock.InOrder(
				mockKubeClient.
					EXPECT().
					Get(context.TODO(), types.NamespacedName{Name: srName, Namespace: srNamespace}, &v1beta1.SpecialResource{}).
					Do(func(_ context.Context, _ types.NamespacedName, _ *v1beta1.SpecialResource) {
						sr.Status.State = newState
					}),
				mockKubeClient.EXPECT().StatusUpdate(context.TODO(), sr),
			)

			state.NewStatusUpdater(mockKubeClient).UpdateWithState(context.TODO(), sr, newState)
		})
	})
})

type conditionExclusivityMatcher struct {
	onlyConditionToBeTrue string
}

func (c conditionExclusivityMatcher) Matches(arg interface{}) bool {
	sr := arg.(*v1beta1.SpecialResource)

	for _, cond := range sr.Status.Conditions {
		if cond.Type == c.onlyConditionToBeTrue {
			if cond.Status != metav1.ConditionTrue {
				return false
			}
		} else {
			if cond.Status == metav1.ConditionTrue {
				return false
			}
		}
	}

	return true
}

func (c conditionExclusivityMatcher) String() string {
	return c.onlyConditionToBeTrue
}

var _ = Describe("SetAs{Ready,Progressing,Errored}", func() {
	const (
		name      = "sr-name"
		namespace = "sr-namespace"
	)

	var (
		kubeClient *clients.MockClientsInterface
		sr         *v1beta1.SpecialResource
		nn         = types.NamespacedName{Name: name, Namespace: namespace}
		justName   = types.NamespacedName{Name: name, Namespace: ""}
	)

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		kubeClient = clients.NewMockClientsInterface(ctrl)
		sr = &v1beta1.SpecialResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
	})

	DescribeTable("will not update the status",
		func(mockSetup func()) {
			mockSetup()
			Expect(state.NewStatusUpdater(kubeClient).SetAsReady(context.TODO(), sr, "Ready", "Ready")).NotTo(Succeed())
		},
		Entry("object not found", func() {
			kubeClient.EXPECT().
				Get(context.TODO(), nn, &v1beta1.SpecialResource{}).
				Return(k8serrors.NewNotFound(v1.Resource("specialresources"), name))
		}),
		Entry("other error during update", func() {
			gomock.InOrder(
				kubeClient.EXPECT().
					Get(context.TODO(), nn, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*v1beta1.SpecialResource)
						sr.DeepCopyInto(o)
						return nil
					}),
				kubeClient.EXPECT().
					StatusUpdate(context.TODO(), gomock.Any()).
					Return(k8serrors.NewBadRequest(name)),
			)
		}),
		Entry("conflict - object deleted", func() {
			gomock.InOrder(
				kubeClient.EXPECT().
					Get(context.TODO(), nn, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*v1beta1.SpecialResource)
						sr.DeepCopyInto(o)
						return nil
					}),
				kubeClient.EXPECT().
					StatusUpdate(context.TODO(), gomock.Any()).
					Return(k8serrors.NewConflict(v1.Resource("specialresources"), name, nil)),
				kubeClient.EXPECT().
					Get(context.TODO(), justName, gomock.Any()).
					Return(k8serrors.NewNotFound(v1.Resource("specialresources"), name)),
			)
		}),
		Entry("conflict - object has DeletionTimestamp", func() {
			gomock.InOrder(
				kubeClient.EXPECT().
					Get(context.TODO(), nn, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*v1beta1.SpecialResource)
						sr.DeepCopyInto(o)
						return nil
					}),
				kubeClient.EXPECT().
					StatusUpdate(context.TODO(), gomock.Any()).
					Return(k8serrors.NewConflict(v1.Resource("specialresources"), name, nil)),
				kubeClient.EXPECT().
					Get(context.TODO(), justName, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						s := obj.(*v1beta1.SpecialResource)
						t := metav1.Unix(0, 0)
						s.SetDeletionTimestamp(&t)
						return nil
					}),
			)
		}),
		Entry("conflict - other error", func() {
			gomock.InOrder(
				kubeClient.EXPECT().
					Get(context.TODO(), nn, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
						o := obj.(*v1beta1.SpecialResource)
						sr.DeepCopyInto(o)
						return nil
					}),
				kubeClient.EXPECT().
					StatusUpdate(context.TODO(), gomock.Any()).
					Return(k8serrors.NewConflict(v1.Resource("specialresources"), name, nil)),
				kubeClient.EXPECT().
					Get(context.TODO(), justName, gomock.Any()).
					Return(k8serrors.NewBadRequest(name)),
			)
		}),
	)

	DescribeTable("Setting one condition to true, should set others to false", func(expectedType string, call func(state.StatusUpdater) error) {
		gomock.InOrder(
			kubeClient.EXPECT().
				Get(context.TODO(), nn, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
					o := obj.(*v1beta1.SpecialResource)
					sr.DeepCopyInto(o)
					return nil
				}),
			kubeClient.EXPECT().
				StatusUpdate(context.TODO(), conditionExclusivityMatcher{expectedType}).
				Return(nil),
		)

		Expect(call(state.NewStatusUpdater(kubeClient))).To(Succeed())
	},
		Entry("Ready", "Ready", func(su state.StatusUpdater) error { return su.SetAsReady(context.Background(), sr, "x", "x") }),
		Entry("Errored", "Errored", func(su state.StatusUpdater) error { return su.SetAsErrored(context.Background(), sr, "x", "x") }),
		Entry("Progressing", "Progressing", func(su state.StatusUpdater) error { return su.SetAsProgressing(context.Background(), sr, "x", "x") }),
	)
})
