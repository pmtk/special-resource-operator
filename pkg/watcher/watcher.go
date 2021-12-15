// Package watcher provides a Watcher which can be used to observe extra resources at runtime.
package watcher

import (
	"errors"
	"fmt"
	"strings"

	srov1beta1 "github.com/openshift-psap/special-resource-operator/api/v1beta1"
	"github.com/openshift-psap/special-resource-operator/pkg/color"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type WatchedResource struct {
	ApiVersion string
	Kind       string
	Name       string
	Namespace  string
}

// pathData contains last known value of property and a list of Namespaced Names (SpecialResourceModules)
// that should be reconciled if that `Data` changes
type pathData struct {
	Data            string
	NamespacedNames []types.NamespacedName
}

type Path = string

// pathToNNSetMap maps Resource's Path to a slice of NamespacedName
type pathToNNSetMap = map[Path]pathData

// watchedResourcesMap maps WatchedResources to Paths which will trigger reconciliation specific of NamespacedNames
type watchedResourcesMap = map[WatchedResource]pathToNNSetMap

func WatchedResourceFromSRM(srm srov1beta1.SpecialResourceModuleWatch) WatchedResource {
	return WatchedResource{
		ApiVersion: srm.ApiVersion,
		Kind:       srm.Kind,
		Name:       srm.Name,
		Namespace:  srm.Namespace,
	}
}

func watchedResourceToUnstructured(wr WatchedResource) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(wr.ApiVersion)
	u.SetKind(wr.Kind)
	return u
}

func getAPIVersion(gvk schema.GroupVersionKind) string {
	if gvk.Group != "" {
		return fmt.Sprintf("%s/%s", gvk.Group, gvk.Version)
	} else {
		return gvk.Version
	}
}

type Watcher interface {
	// AddResourceToWatch registers given WatchedResource to trigger Reconciliation of CR specified by a NamespacedName
	AddResourceToWatch(srov1beta1.SpecialResourceModuleWatch, types.NamespacedName) error

	// ReconcileWatches is a general function that updates list of watched resources based on given SpecialResourceModule
	ReconcileWatches(srov1beta1.SpecialResourceModule) error

	// TODO: Add mechanism to handle change of CR's WatchedResources (e.g. change due to user's )
	// RemoveResourceFromWatch(WatchedResource) error
	// RemoveWatchForCR(types.NamespacedName) error
}

func New(ctrl controller.Controller) Watcher {
	return &watcher{
		log:              zap.New(zap.UseDevMode(true)).WithName(color.Print("watcher", color.Blue)),
		ctrl:             ctrl,
		watchedResources: make(map[WatchedResource]map[string]pathData),
	}
}

type watcher struct {
	log              logr.Logger
	ctrl             controller.Controller
	watchedResources watchedResourcesMap
}

func (w *watcher) ReconcileWatches(srm srov1beta1.SpecialResourceModule) error {
	return nil
}

func (w *watcher) AddResourceToWatch(r srov1beta1.SpecialResourceModuleWatch, nnToTrigger types.NamespacedName) error {
	if w == nil {
		return errors.New("watcher is not initialized")
	}

	l := w.log.WithValues("resource", r, "triggers", nnToTrigger)
	l.Info("adding resource to be watched")

	wr := WatchedResource{
		ApiVersion: r.ApiVersion,
		Kind:       r.Kind,
		Name:       r.Name,
		Namespace:  r.Namespace,
	}

	if w.isAlreadyBeingWatched(wr, r.Path, nnToTrigger) {
		l.Info("resource is already being watched")
		return nil
	}

	// Predicates are not utilized because client.Object inside a predicate func does not have enough context.
	// Multiple SRMs can depend on different paths of a single watched resource.
	// Inside predicate func, it's not known which SRM will be triggered and therefore which path to check.
	// Because of that reason, mapping function takes care of filtering out.
	if err := w.ctrl.Watch(
		&source.Kind{Type: watchedResourceToUnstructured(wr)},
		handler.EnqueueRequestsFromMapFunc(w.mapper) /*, w.genPredicates() */); err != nil {

		l.Error(err, "failed to start watching a resource")
		return err
	}

	// TODO: Potential race? Registering first, then adding to a map = mapper func might get invoked before adding to a map?
	w.addToWatched(wr, r.Path, nnToTrigger)

	return nil
}

// func (w *watcher) RemoveResourceFromWatch(WatchedResource) error {
// 	// TODO
// 	return nil
// }

func (w *watcher) isAlreadyBeingWatched(wr WatchedResource, path string, nnToTrigger types.NamespacedName) bool {

	if paths, ok := w.watchedResources[wr]; ok {
		if pathData, ok := paths[path]; ok {
			for _, nn := range pathData.NamespacedNames {
				if nn == nnToTrigger {
					return true
				}
			}
		}
	}

	return false
}

func (w *watcher) addToWatched(wr WatchedResource, path string, nnToTrigger types.NamespacedName) {

	var ok bool
	var paths map[string]pathData
	if paths, ok = w.watchedResources[wr]; !ok {
		paths = make(map[string]pathData)
	}

	var pd pathData
	if pd, ok = paths[path]; !ok {
		pd = pathData{
			NamespacedNames: make([]types.NamespacedName, 0),
		}
	}

	pd.NamespacedNames = append(pd.NamespacedNames, nnToTrigger)
	paths[path] = pd
	w.watchedResources[wr] = paths
}

// mapper returns a list of NamespacedNames based on given Object.
//
// If Object's apiVersion+kind+name+namespace are registered to be watched,
// the paths in that object are inspected for changes.
// If the value in incoming object is different than stored value,
// a reconciliation will be triggered for SpecialResourceModules that depend on that object's value at path.
func (w *watcher) mapper(o client.Object) []reconcile.Request {
	gvk := o.GetObjectKind().GroupVersionKind()
	apiVer := getAPIVersion(gvk)

	wrObj := WatchedResource{
		ApiVersion: apiVer,
		Kind:       gvk.Kind,
		Name:       o.GetName(),
		Namespace:  o.GetNamespace(),
	}

	unstr := o.(*unstructured.Unstructured)

	crsToTrigger := []reconcile.Request{}

	if wr, ok := w.watchedResources[wrObj]; ok {
		for path, pathData := range wr {

			// TODO: Substitute with proper jsonpath?
			val, found, err := unstructured.NestedString(unstr.Object, strings.Split(path, ".")...)
			if err != nil {
				w.log.Error(err, "failed to get nested string", "resource", wr)
				continue
			}
			if !found {
				w.log.Info("could not obtain property at specified path", "path", path, "resource", wr)
				continue
			}
			w.log.Info("obtained value @ path", "path", path, "value", val)

			if pathData.Data != val {
				for _, nn := range pathData.NamespacedNames {
					crsToTrigger = append(crsToTrigger, reconcile.Request{NamespacedName: nn})
				}
			} else {
				w.log.Info("data did not change - no retrigger")
			}

			// Update pathData.Data (which stores last known value specified by a `path`)
			pathData.Data = val
			wr[path] = pathData
			w.watchedResources[wrObj] = wr
		}
	}

	if len(crsToTrigger) > 0 {
		w.log.Info("watched resource will trigger reconciliation of CRs", "resource", wrObj, "crs", crsToTrigger)
	}

	return crsToTrigger
}

func (w *watcher) genPredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(e event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		UpdateFunc:  func(e event.UpdateEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return true },
	}
}
