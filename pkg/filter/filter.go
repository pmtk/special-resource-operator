package filter

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/openshift-psap/special-resource-operator/pkg/color"
	"github.com/openshift-psap/special-resource-operator/pkg/hash"
	"github.com/openshift-psap/special-resource-operator/pkg/kernel"
	"github.com/openshift-psap/special-resource-operator/pkg/lifecycle"
	"github.com/openshift-psap/special-resource-operator/pkg/storage"
	"github.com/openshift-psap/special-resource-operator/pkg/warn"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	sroGVK = "SpecialResource"
	owned  = "specialresource.openshift.io/owned"
)

var (
	log = zap.New(zap.UseDevMode(true)).WithName(color.Print("filter", color.Purple))
)

func SetLabel(obj *unstructured.Unstructured) error {

	var labels map[string]string

	if labels = obj.GetLabels(); labels == nil {
		labels = make(map[string]string)
	}

	// TODO: Parametrize label to apply (if this function gets used by an alternative controller (for SpecialResourceModule))
	labels[owned] = "true"
	obj.SetLabels(labels)

	return setSubResourceLabel(obj)
}

func setSubResourceLabel(obj *unstructured.Unstructured) error {

	if obj.GetKind() == "DaemonSet" || obj.GetKind() == "Deployment" ||
		obj.GetKind() == "StatefulSet" {

		labels, found, err := unstructured.NestedMap(obj.Object, "spec", "template", "metadata", "labels")
		if err != nil {
			return err
		}
		if !found {
			return errors.New("Labels not found")
		}

		labels[owned] = "true"
		if err := unstructured.SetNestedMap(obj.Object, labels, "spec", "template", "metadata", "labels"); err != nil {
			return err
		}
	}

	if obj.GetKind() == "BuildConfig" {
		log.Info("TODO: how to set label ownership for Builds and related Pods")
		/*
			output, found, err := unstructured.NestedMap(obj.Object, "spec", "output")
			if err != nil {
				return err
			}
			if !found {
				return errors.New("output not found")
			}

			label := make(map[string]interface{})
			label["name"] = owned
			label["value"] = "true"
			imageLabels := append(make([]interface{}, 0), label)

			if _, found := output["imageLabels"]; !found {
				err := unstructured.SetNestedSlice(obj.Object, imageLabels, "spec", "output", "imageLabels")
				if err != nil {
					return err
				}
			}
		*/
	}
	return nil
}

type Filter interface {
	// SetOwnedLabel(obj *unstructured.Unstructured) error
	GetPredicates() predicate.Predicate
	GetMode() string
}

func NewFilter(gvk, ownedLabel string) Filter {
	return &filter{
		log:        zap.New(zap.UseDevMode(true)).WithName(color.Print("filter", color.Purple)),
		gvk:        gvk,
		ownedLabel: ownedLabel,
	}
}

type filter struct {
	log        logr.Logger
	gvk        string
	ownedLabel string

	mode string
}

func (f *filter) GetMode() string {
	return f.mode
}

func (f *filter) isSpecialResource(obj client.Object) bool {

	kind := obj.GetObjectKind().GroupVersionKind().Kind

	if kind == f.gvk {
		f.log.Info(f.mode+" IsSpecialResource (sroGVK)", "Name", obj.GetName(), "Type", reflect.TypeOf(obj).String())
		return true
	}

	t := reflect.TypeOf(obj).String()

	if strings.Contains(t, f.gvk) {
		f.log.Info(f.mode+" IsSpecialResource (reflect)", "Name", obj.GetName(), "Type", reflect.TypeOf(obj).String())
		return true

	}

	// If SRO owns the resource than it cannot be a SpecialResource
	if f.owned(obj) {
		return false
	}

	// We need this because a newly created SpecialResource will not yet
	// have a GVK
	selfLink := obj.GetSelfLink()
	if strings.Contains(selfLink, "/apis/sro.openshift.io/v") {
		f.log.Info(f.mode+" IsSpecialResource (selflink)", "Name", obj.GetName(), "Type", reflect.TypeOf(obj).String())
		return true
	}
	if kind == "" {
		objstr := fmt.Sprintf("%+v", obj)
		if strings.Contains(objstr, "sro.openshift.io/v") {
			f.log.Info(f.mode+" IsSpecialResource (contains)", "Name", obj.GetName(), "Type", reflect.TypeOf(obj).String())
			return true
		}
	}

	return false
}

func (f *filter) owned(obj client.Object) bool {

	for _, owner := range obj.GetOwnerReferences() {
		if owner.Kind == f.gvk {
			f.log.Info(f.mode+" Owned (sroGVK)", "Name", obj.GetName(),
				"Type", reflect.TypeOf(obj).String())
			return true
		}
	}

	var labels map[string]string

	if labels = obj.GetLabels(); labels != nil {
		if _, found := labels[f.ownedLabel]; found {
			f.log.Info(f.mode+" Owned (label)", "Name", obj.GetName(),
				"Type", reflect.TypeOf(obj).String())
			return true
		}
	}
	return false
}

func (f *filter) GetPredicates() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {

			f.mode = "CREATE"
			// If a specialresource dependency is deleted we
			/* want to recreate it so handle the delete event */
			obj := e.Object

			if f.isSpecialResource(obj) {
				return true
			}

			if f.owned(obj) {
				return true
			}

			return false
		},

		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates if the resourceVersion does not change
			// resourceVersion is updated when the object is modified

			/* UPDATING THE STATUS WILL INCREASE THE RESOURCEVERSION DISABLING
			 * BUT KEEPING FOR REFERENCE
			if e.MetaOld.GetResourceVersion() == e.MetaNew.GetResourceVersion() {
				return false
			}*/
			f.mode = "UPDATE"

			e.ObjectOld.GetGeneration()
			e.ObjectOld.GetOwnerReferences()

			obj := e.ObjectNew

			// Required for the case when pods are deleted due to OS upgrade

			if f.owned(obj) && kernel.IsObjectAffine(obj) {
				if e.ObjectOld.GetGeneration() == e.ObjectNew.GetGeneration() &&
					e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
					return false
				} else {
					f.log.Info(f.mode+" Owned Generation or resourceVersion Changed for kernel affine object",
						"Name", obj.GetName(), "Type", reflect.TypeOf(obj).String())
					if reflect.TypeOf(obj).String() == "*v1.DaemonSet" && e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() {
						err := lifecycle.UpdateDaemonSetPods(obj)
						warn.OnError(err)
					}
					return true
				}
			}

			// Ignore updates to CR status in which case metadata.Generation does not change
			if e.ObjectOld.GetGeneration() == e.ObjectNew.GetGeneration() {
				return false
			}
			// Some objects will increase generation on Update SRO sets the
			// resourceversion New = Old so we can filter on those even if an
			// update does not change anything see e.g. Deployment or SCC
			if e.ObjectOld.GetResourceVersion() == e.ObjectNew.GetResourceVersion() {
				return false
			}

			// If a specialresource dependency is updated we
			// want to reconcile it, handle the update event

			if f.isSpecialResource(obj) {
				f.log.Info(f.mode+" IsSpecialResource GenerationChanged",
					"Name", obj.GetName(), "Type", reflect.TypeOf(obj).String())
				return true
			}

			// If we do not own the object, do not care
			if f.owned(obj) {

				f.log.Info(f.mode+" Owned GenerationChanged",
					"Name", obj.GetName(), "Type", reflect.TypeOf(obj).String())

				if reflect.TypeOf(obj).String() == "*v1.DaemonSet" {
					err := lifecycle.UpdateDaemonSetPods(obj)
					warn.OnError(err)
				}

				return true
			}

			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {

			f.mode = "DELETE"
			// If a specialresource dependency is deleted we
			/* want to recreate it so handle the delete event */
			obj := e.Object
			if f.isSpecialResource(obj) {
				return true
			}

			// If we do not own the object, do not care
			if f.owned(obj) {

				ins := types.NamespacedName{
					Namespace: os.Getenv("OPERATOR_NAMESPACE"),
					Name:      "special-resource-lifecycle",
				}
				key, err := hash.FNV64a(obj.GetNamespace() + obj.GetName())
				if err != nil {
					warn.OnError(err)
					return false
				}
				err = storage.DeleteConfigMapEntry(key, ins)
				warn.OnError(err)

				return true
			}
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {

			f.mode = "GENERIC"

			// If a specialresource dependency is updated we
			// want to reconcile it, handle the update event
			obj := e.Object
			if f.isSpecialResource(obj) {
				return true
			}
			// If we do not own the object, do not care
			if f.owned(obj) {
				return true
			}
			return false

		},
	}
}
