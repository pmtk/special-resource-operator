package state

import (
	"context"

	"github.com/go-logr/logr"
	"github.com/openshift-psap/special-resource-operator/api/v1beta1"
	"github.com/openshift-psap/special-resource-operator/pkg/clients"
	"github.com/openshift-psap/special-resource-operator/pkg/utils"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	ready       = "SpecialResourceIsReady"
	progressing = "Progressing"
	errored     = "ErrorHasOccurred"
)

//go:generate mockgen -source=statusupdater.go -package=state -destination=mock_statusupdater_api.go

type StatusUpdater interface {
	UpdateWithState(context.Context, *v1beta1.SpecialResource, string)
	SetAsReady(ctx context.Context, sr *v1beta1.SpecialResource, reason, message string) error
	SetAsProgressing(ctx context.Context, sr *v1beta1.SpecialResource, reason, message string) error
	SetAsErrored(ctx context.Context, sr *v1beta1.SpecialResource, reason, message string) error
}

type statusUpdater struct {
	kubeClient clients.ClientsInterface
	log        logr.Logger
}

func NewStatusUpdater(kubeClient clients.ClientsInterface) StatusUpdater {
	return &statusUpdater{
		kubeClient: kubeClient,
		log:        ctrl.Log.WithName(utils.Print("status-updater", utils.Blue)),
	}
}

func (su *statusUpdater) SetAsProgressing(ctx context.Context, sr *v1beta1.SpecialResource, reason, message string) error {
	return su.updateWithMutator(ctx, sr, func(o *v1beta1.SpecialResource) {
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceProgressing, Status: metav1.ConditionTrue, Reason: reason, Message: message})
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceReady, Status: metav1.ConditionFalse, Reason: progressing})
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceErrored, Status: metav1.ConditionFalse, Reason: progressing})
	})
}

func (su *statusUpdater) SetAsReady(ctx context.Context, sr *v1beta1.SpecialResource, reason, message string) error {
	return su.updateWithMutator(ctx, sr, func(o *v1beta1.SpecialResource) {
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceReady, Status: metav1.ConditionTrue, Reason: reason, Message: message})

		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceProgressing, Status: metav1.ConditionFalse, Reason: ready})
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceErrored, Status: metav1.ConditionFalse, Reason: ready})
	})
}

func (su *statusUpdater) SetAsErrored(ctx context.Context, sr *v1beta1.SpecialResource, reason, message string) error {
	return su.updateWithMutator(ctx, sr, func(o *v1beta1.SpecialResource) {
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceErrored, Status: metav1.ConditionTrue, Reason: reason, Message: message})

		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceReady, Status: metav1.ConditionFalse, Reason: errored})
		meta.SetStatusCondition(&o.Status.Conditions, metav1.Condition{Type: v1beta1.SpecialResourceProgressing, Status: metav1.ConditionFalse, Reason: errored})
	})
}

func (su *statusUpdater) updateWithMutator(ctx context.Context, sr *v1beta1.SpecialResource, mutator func(sr *v1beta1.SpecialResource)) error {

	update := v1beta1.SpecialResource{}

	// If we cannot find the SR than something bad is going on ..
	objectKey := types.NamespacedName{Name: sr.GetName(), Namespace: sr.GetNamespace()}
	err := su.kubeClient.Get(ctx, objectKey, &update)
	if err != nil {
		return errors.Wrap(err, "Is SR being deleted? Cannot get current instance")
	}

	if sr.Status.Conditions == nil {
		sr.Status.Conditions = make([]metav1.Condition, 0)
	}

	mutator(&update)
	update.DeepCopyInto(sr)

	err = su.kubeClient.StatusUpdate(ctx, sr)
	if apierrors.IsConflict(err) {
		objectKey := types.NamespacedName{Name: sr.Name, Namespace: ""}
		err := su.kubeClient.Get(ctx, objectKey, sr)
		if apierrors.IsNotFound(err) {
			return errors.New("Could not update status because the object does not exist")
		}

		// Do not update the status if we're in the process of being deleted
		isMarkedToBeDeleted := sr.GetDeletionTimestamp() != nil
		if isMarkedToBeDeleted {
			return errors.New("Status won't be updated because object is marked for deletion")
		}

		return errors.New("Conflict occurred during status update")
	}

	if err != nil {
		return errors.Wrap(err, "Failed to update SpecialResource status")
	}

	return nil
}

// UpdateWithState updates sr's Status.State property with state, and updates the object in Kubernetes.
// TODO(qbarrand) make this function return an error
func (su *statusUpdater) UpdateWithState(ctx context.Context, sr *v1beta1.SpecialResource, state string) {

	update := v1beta1.SpecialResource{}

	// If we cannot find the SR than something bad is going on ..
	objectKey := types.NamespacedName{Name: sr.GetName(), Namespace: sr.GetNamespace()}
	err := su.kubeClient.Get(ctx, objectKey, &update)
	if err != nil {
		utils.WarnOnError(errors.Wrap(err, "Is SR being deleted? Cannot get current instance"))
		return
	}

	update.Status.State = state
	update.DeepCopyInto(sr)

	err = su.kubeClient.StatusUpdate(ctx, sr)
	if apierrors.IsConflict(err) {
		objectKey := types.NamespacedName{Name: sr.Name, Namespace: ""}
		err := su.kubeClient.Get(ctx, objectKey, sr)
		if apierrors.IsNotFound(err) {
			return
		}
		// Do not update the status if we're in the process of being deleted
		isMarkedToBeDeleted := sr.GetDeletionTimestamp() != nil
		if isMarkedToBeDeleted {
			return
		}

	}

	if err != nil {
		su.log.Error(err, "Failed to update SpecialResource status")
		return
	}
}
