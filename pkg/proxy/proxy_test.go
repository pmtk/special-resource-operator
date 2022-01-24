package proxy_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-psap/special-resource-operator/internal/mocks"
	"github.com/openshift-psap/special-resource-operator/pkg/proxy"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	httpProxy     = "http-host-with-proxy"
	httpsProxy    = "https-host-with-proxy"
	noProxy       = "host-without-proxy"
	trustedCAName = "trusted-ca-name"
)

func TestProxy(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Proxy Suite")
}

var _ = Describe("Setup", func() {
	var (
		ctrl       *gomock.Controller
		mockClient *mocks.MockClientsInterface
		p          proxy.ProxyAPI
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockClient = mocks.NewMockClientsInterface(ctrl)
		p = proxy.NewProxyAPI(mockClient)

		mockClient.EXPECT().
			HasResource(configv1.SchemeGroupVersion.WithResource("proxies")).
			Return(true, nil).
			AnyTimes()
		mockClient.EXPECT().
			List(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, obj client.ObjectList, _ ...client.ListOption) error {
				proxyCfg := unstructured.Unstructured{}
				proxyCfg.SetName("cluster")
				Expect(unstructured.SetNestedField(proxyCfg.Object, httpProxy, "spec", "httpProxy")).To(Succeed())
				Expect(unstructured.SetNestedField(proxyCfg.Object, httpsProxy, "spec", "httpsProxy")).To(Succeed())
				Expect(unstructured.SetNestedField(proxyCfg.Object, noProxy, "spec", "noProxy")).To(Succeed())
				Expect(unstructured.SetNestedField(proxyCfg.Object, trustedCAName, "spec", "trustedCA", "name")).To(Succeed())

				u := obj.(*unstructured.UnstructuredList)
				u.Items = append(u.Items, proxyCfg)

				return nil
			}).AnyTimes()
	})

	It("should return an error for Pod with empty spec", func() {
		pod := v1.Pod{
			TypeMeta: metav1.TypeMeta{Kind: "Pod"},
		}

		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
		Expect(err).NotTo(HaveOccurred())

		uo := unstructured.Unstructured{Object: m}

		err = p.Setup(&uo)
		Expect(err).To(HaveOccurred())
	})

	It("should return no error for Pod with one container", func() {
		pod := v1.Pod{
			TypeMeta: metav1.TypeMeta{Kind: "Pod"},
			Spec: v1.PodSpec{
				Containers: []v1.Container{
					{
						Name: "test",
						Env:  make([]v1.EnvVar, 0),
					},
				},
			},
		}

		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
		Expect(err).NotTo(HaveOccurred())

		uo := unstructured.Unstructured{Object: m}

		_, err = p.ClusterConfiguration(context.Background())
		Expect(err).NotTo(HaveOccurred())

		err = p.Setup(&uo)
		Expect(err).NotTo(HaveOccurred())

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(uo.Object, &pod)
		Expect(err).NotTo(HaveOccurred())

		// TODO(qbarrand) fix the method and then uncomment.
		// SetupPod does not set the resulting containers slice with unstructured.SetNestedSlice
		//env := pod.Spec.Containers[0].Env

		//assert.Contains(t, env, v1.EnvVar{Name: "HTTP_PROXY", Value: httpProxy})
		//assert.Contains(t, env, v1.EnvVar{Name: "HTTPS_PROXY", Value: httpsProxy})
		//assert.Contains(t, env, v1.EnvVar{Name: "NO_PROXY", Value: noProxy})
	})

	It("should return an error for DaemonSet with empty spec", func() {
		ds := appsv1.DaemonSet{
			TypeMeta: metav1.TypeMeta{Kind: "DaemonSet"},
		}

		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ds)
		Expect(err).NotTo(HaveOccurred())

		uo := unstructured.Unstructured{Object: m}

		_, err = p.ClusterConfiguration(context.Background())
		Expect(err).NotTo(HaveOccurred())

		err = p.Setup(&uo)
		Expect(err).To(HaveOccurred())
	})

	It("should return no error for DaemonSet with one container template", func() {
		ds := appsv1.DaemonSet{
			TypeMeta: metav1.TypeMeta{Kind: "DaemonSet"},
			Spec: appsv1.DaemonSetSpec{
				Template: v1.PodTemplateSpec{
					Spec: v1.PodSpec{
						Containers: []v1.Container{
							{
								Name: "test",
								Env:  make([]v1.EnvVar, 0),
							},
						},
					},
				},
			},
		}

		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&ds)
		Expect(err).NotTo(HaveOccurred())

		uo := unstructured.Unstructured{Object: m}

		_, err = p.ClusterConfiguration(context.Background())
		Expect(err).NotTo(HaveOccurred())

		err = p.Setup(&uo)
		Expect(err).NotTo(HaveOccurred())

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(uo.Object, &ds)
		Expect(err).NotTo(HaveOccurred())

		// TODO(qbarrand) fix the method and then uncomment.
		// SetupDaemonSet does not set the resulting containers slice with unstructured.SetNestedSlice
		//env := ds.Spec.Template.Spec.Containers[0].Env

		//assert.Contains(t, env, v1.EnvVar{Name: "HTTP_PROXY", Value: httpProxy})
		//assert.Contains(t, env, v1.EnvVar{Name: "HTTPS_PROXY", Value: httpsProxy})
		//assert.Contains(t, env, v1.EnvVar{Name: "NO_PROXY", Value: noProxy})
	})
})

// TODO(qbarrand) make the DiscoveryClient in clients.HasResource injectable, so we can mock it.
var _ = Describe("ClusterConfiguration", func() {
	var (
		ctrl           *gomock.Controller
		p              proxy.ProxyAPI
		mockKubeClient *mocks.MockClientsInterface
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		mockKubeClient = mocks.NewMockClientsInterface(ctrl)
		p = proxy.NewProxyAPI(mockKubeClient)
	})

	It("HasResource failed", func() {
		mockKubeClient.EXPECT().HasResource(gomock.Any()).Times(1).Return(false, fmt.Errorf("some error"))
		_, err := p.ClusterConfiguration(context.TODO())
		Expect(err).To(HaveOccurred())
	})

	It("Unavailble proxy", func() {
		mockKubeClient.EXPECT().HasResource(gomock.Any()).Times(1).Return(false, nil)
		mockKubeClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(0)
		_, err := p.ClusterConfiguration(context.TODO())
		Expect(err).NotTo(HaveOccurred())
	})

	It("Proxy List failed", func() {
		mockKubeClient.EXPECT().HasResource(gomock.Any()).Times(1).Return(true, nil)
		mockKubeClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).Return(fmt.Errorf("some error"))
		_, err := p.ClusterConfiguration(context.TODO())
		Expect(err).To(HaveOccurred())
	})

	It("Config Proxy", func() {
		mockKubeClient.EXPECT().HasResource(gomock.Any()).Times(1).Return(true, nil)
		mockKubeClient.EXPECT().List(gomock.Any(), gomock.Any()).Times(1).Return(nil)
		proxy, err := p.ClusterConfiguration(context.TODO())
		Expect(err).NotTo(HaveOccurred())
		Expect(proxy.HttpProxy).To(BeEmpty())
		Expect(proxy.HttpsProxy).To(BeEmpty())
		Expect(proxy.NoProxy).To(BeEmpty())
		Expect(proxy.TrustedCA).To(BeEmpty())
	})

})
