/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	srov1beta1 "github.com/openshift-psap/special-resource-operator/api/v1beta1"
	"github.com/openshift-psap/special-resource-operator/pkg/clients"
	"github.com/openshift-psap/special-resource-operator/pkg/color"
	"github.com/openshift-psap/special-resource-operator/pkg/filter"
	"github.com/openshift-psap/special-resource-operator/pkg/watcher"
	buildv1 "github.com/openshift/api/build/v1"
	"github.com/pkg/errors"

	imagev1 "github.com/openshift/api/image/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	SRMgvk        = "SpecialResourceModule"
	SRMOwnedLabel = "specialresourcemodule.openshift.io/owned"
)

func createImageStream(name, namespace string) error {
	is := imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				SRMOwnedLabel: "true",
			},
		},
		Spec: imagev1.ImageStreamSpec{},
	}
	_, err := ctrl.CreateOrUpdate(context.TODO(), clients.Interface.GetClient(), &is, noop)
	return err
}

type OCPVersionInfo struct {
	KernelVersion   string
	RTKernelVersion string
	DTKImage        string
}

func getOCPVersions(watchList []srov1beta1.SpecialResourceModuleWatch) map[string]OCPVersionInfo {
	//TODO
	return nil
}

func FindSRM(a []srov1beta1.SpecialResourceModule, x string) (int, bool) {
	for i, n := range a {
		if x == n.GetName() {
			return i, true
		}
	}
	return -1, false
}

// SpecialResourceModuleReconciler reconciles a SpecialResource object
type SpecialResourceModuleReconciler struct {
	Log    logr.Logger
	Scheme *runtime.Scheme
	Filter filter.Filter
	// ctrl   controller.Controller

	watcher watcher.Watcher

	// watchedResources map[srov1beta1.SpecialResourceModuleWatch]types.NamespacedName
}

// func (r *SpecialResourceModuleReconciler) addToWatch(srm srov1beta1.SpecialResourceModule, resource srov1beta1.SpecialResourceModuleWatch) error {
// 	r.Log.Info("adding resource to watch", "resource", resource)

// 	if r.watchedResources == nil {
// 		r.watchedResources = make(map[srov1beta1.SpecialResourceModuleWatch]types.NamespacedName)
// 	}

// 	if _, ok := r.watchedResources[resource]; ok {
// 		r.Log.Info("resource to watch already being watched!")
// 		return nil
// 	}

// 	/*
// 		TODO (if needed):
// 		- allowing multiple SpecialResourceModule to be triggered by the same watched resource
// 		  (map[srov1beta1.SpecialResourceModuleWatch][]types.NamespacedName)
// 		- reacting on change of SRM's watched resources, i.e. SRM's watches changes - old binding must be removed
// 		- remove Watch on deleted CR - to stop watching for types, otherwise the list will grow
// 	*/

// 	// NOTE: To observe the CR, the CRD must be installed first.

// 	typeToWatch := &unstructured.Unstructured{}
// 	typeToWatch.SetAPIVersion(resource.ApiVersion)
// 	typeToWatch.SetKind(resource.Kind)
// 	typeToWatch.SetName(resource.Name)

// 	r.watchedResources[resource] = types.NamespacedName{Namespace: srm.Namespace, Name: srm.Name}

// 	// f returns a NamespacedName (SRM) to be reconciled for incoming Object
// 	f := func(o client.Object) []reconcile.Request {
// 		gvk := o.GetObjectKind().GroupVersionKind()
// 		r.Log.Info("matcher for watched objects", "gvk", gvk, "name", o.GetName(), "ns", o.GetNamespace())

// 		for k, v := range r.watchedResources {
// 			apiVer := ""
// 			if gvk.Group != "" {
// 				apiVer = fmt.Sprintf("%s/%s", gvk.Group, gvk.Version)
// 			} else {
// 				apiVer = gvk.Version
// 			}

// 			if k.Kind == gvk.Kind && k.ApiVersion == apiVer && k.Name == o.GetName() && k.Namespace == o.GetNamespace() {
// 				// NOTE: Alternatively, name, namespace & changes to property @ Path could be filtered out using predicates
// 				r.Log.Info("found a SRM for incoming watched resource", "resource", k, "special-resource-module", v)
// 				return []reconcile.Request{{NamespacedName: v}}
// 			}
// 		}
// 		return []reconcile.Request{}
// 	}

// 	return r.ctrl.Watch(&source.Kind{Type: typeToWatch}, handler.EnqueueRequestsFromMapFunc(f) /*, predicate.Predicate...*/)
// }

// Reconcile Reconiliation entry point
func (r *SpecialResourceModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := r.Log.WithName(color.Print("reconcile: "+req.Name, color.Purple))
	l.Info("Reconciling")

	srm := &srov1beta1.SpecialResourceModuleList{}

	opts := []client.ListOption{}
	err := clients.Interface.List(context.Background(), srm, opts...)
	if err != nil {
		return reconcile.Result{}, err
	}

	var request int
	var found bool
	if request, found = FindSRM(srm.Items, req.Name); !found {
		return reconcile.Result{}, fmt.Errorf("%s not found", req.Name)
	}
	resource := srm.Items[request]

	if !resource.Status.ImageStreamCreated {
		if err := createImageStream(req.Name, resource.Spec.Namespace); err != nil {
			return reconcile.Result{}, err
		}
		resource.Status.ImageStreamCreated = true
	}

	for _, watchElement := range resource.Spec.Watch {
		r.watcher.AddResourceToWatch(watchElement, types.NamespacedName{Name: resource.Name, Namespace: resource.Namespace})
	}

	clusterVersions := getOCPVersions(resource.Spec.Watch)
	for resourceVersion, _ := range resource.Status.Versions {
		if _, ok := clusterVersions[resourceVersion]; !ok {
			//TODO not found. Need to remove everything
		} else {
			//TODO Found, need to check for reconcile stage.
		}
	}
	for clusterVersion, _ := range clusterVersions {
		if _, ok := resource.Status.Versions[clusterVersion]; !ok {
			//TODO not found, this version is new. reconcile.
		}
	}
	//TODO update resource status.
	return reconcile.Result{}, nil
}

// SetupWithManager main initalization for manager
func (r *SpecialResourceModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	platform, err := clients.Interface.GetPlatform()
	if err != nil {
		return err
	}

	if platform == "OCP" {
		c, err := ctrl.NewControllerManagedBy(mgr).
			For(&srov1beta1.SpecialResourceModule{}).
			Owns(&imagev1.ImageStream{}).
			Owns(&buildv1.BuildConfig{}).
			WithOptions(controller.Options{
				MaxConcurrentReconciles: 1,
			}).
			// WithEventFilter(predicates(r)).
			WithEventFilter(r.Filter.GetPredicates()).
			Build(r)

		r.watcher = watcher.New(c)
		return err
	}
	return errors.New("SpecialResourceModules only work in OCP")
}
