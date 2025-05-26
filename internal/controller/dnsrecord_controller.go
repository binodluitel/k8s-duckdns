/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"

	"github.com/binodluitel/k8s-duckdns/internal/clients/duckdns"
	"github.com/gogo/protobuf/proto"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metamachinery "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	duckdnsv1alpha1 "github.com/binodluitel/k8s-duckdns/api/v1alpha1"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// DNSRecordReconciler reconciles a DNSRecord object
type DNSRecordReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	ControllerName string
	DuckDNSClient  *duckdns.Client
	Recorder       record.EventRecorder
}

const (
	// DNSRecordControllerName is the name of the controller that manages DNSRecord resources.
	DNSRecordControllerName = "DNSRecordController"

	// DNSRecordFinalizer is the finalizer for DNSRecord resources.
	DNSRecordFinalizer = "finalizer.duckdns.luitel.dev/dnsrecord"
)

// +kubebuilder:rbac:groups=duckdns.luitel.dev,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=duckdns.luitel.dev,resources=dnsrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=duckdns.luitel.dev,resources=dnsrecords/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;update;delete
// +kubebuilder:rbac:groups=apps,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.20.4/pkg/reconcile
func (r *DNSRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.V(1).Info("Reconciling DNSRecord")
	defer logger.V(1).Info("Reconciliation of DNSRecord completed")

	dnsRecord := new(duckdnsv1alpha1.DNSRecord)
	if err := r.Get(ctx, req.NamespacedName, dnsRecord); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !dnsRecord.GetDeletionTimestamp().IsZero() {
		return r.processDeletion(ctx, dnsRecord)
	}

	// Check if the DNSRecord has the finalizer set, if not, add it
	if !ctrlutil.ContainsFinalizer(dnsRecord, DNSRecordFinalizer) {
		logger.Info("Adding finalizer to DNSRecord")

		dnsRecordCopy := dnsRecord.DeepCopy()
		if !ctrlutil.AddFinalizer(dnsRecordCopy, DNSRecordFinalizer) {
			logger.Error(nil, "Failed to add finalizer to DNSRecord")
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"FinalizerAddFailed",
				"Failed to add finalizer %s to DNSRecord %s",
				DNSRecordFinalizer,
				dnsRecord.Name,
			)
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer %s", DNSRecordFinalizer)
		}

		if err := r.Update(ctx, dnsRecordCopy); err != nil {
			logger.Error(err, "Failed to add finalizer", "finalizer", DNSRecordFinalizer)
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer %s", DNSRecordFinalizer)
		}

		r.Recorder.Eventf(
			dnsRecord,
			"Normal",
			"FinalizerAdded",
			"Finalizer %s added to DNSRecord %s",
			DNSRecordFinalizer,
			dnsRecord.Name,
		)

		return ctrl.Result{}, nil
	}

	if dnsRecord.Status.Conditions == nil {
		dnsRecord.Status.Conditions = []metav1.Condition{}
	}

	// If the status is Set to "OrlKorrect", we can skip the DNS update process in DuckDNS.
	// Register the DuckDNS if not already registered.
	// Then create a CronJob to update the DNS record periodically when the IP changes
	if !metamachinery.IsStatusConditionTrue(dnsRecord.Status.Conditions, duckdnsv1alpha1.DNSRecordOrlKorrect.String()) {
		logger.Info("DNSRecord is not OrlKorrect, proceeding with update")
		domains := r.domainsToString(dnsRecord.Spec.Domains)
		requestParams := duckdns.Params{
			Domains: domains,
			IPv4:    dnsRecord.Spec.IPv4Address,
			IPv6:    dnsRecord.Spec.IPv6Address,
			Clear:   dnsRecord.Spec.Clear,
			Txt:     dnsRecord.Spec.Txt,
		}

		// Update the DNS record in DuckDNS
		if err := r.DuckDNSClient.UpdateDNS(ctx, requestParams); err != nil {
			logger.Error(err, "Failed to update DNS record")
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"UpdateFailed",
				"Failed to update DNS record for domains %s: %v",
				domains,
				err,
			)

			dnsRecordCopy := dnsRecord.DeepCopy()
			metamachinery.SetStatusCondition(&dnsRecordCopy.Status.Conditions, metav1.Condition{
				Message:            "DNS record update failed",
				Reason:             "DNSRecordUpdateFailed",
				Status:             metav1.ConditionFalse,
				Type:               duckdnsv1alpha1.DNSRecordOrlKorrect.String(),
				LastTransitionTime: metav1.Now(),
			})

			if err := r.Status().Update(ctx, dnsRecordCopy); err != nil {
				logger.Error(err, "Failed to update DNSRecord status")
				r.Recorder.Eventf(
					dnsRecord,
					"Warning",
					"StatusUpdateFailed",
					"Failed to update status for DNSRecord %s: %v",
					dnsRecord.Name,
					err,
				)
				return ctrl.Result{}, fmt.Errorf("failed to update status for DNSRecord, %w", err)
			}
			return ctrl.Result{}, nil
		}

		// If we are here, it means the DNS record update was successful in DuckDNS.
		// We will update the DNSRecord status to reflect the successful update.
		dnsRecordCopy := dnsRecord.DeepCopy()
		dnsRecordCopy.Status.IPv4Address = dnsRecord.Spec.IPv4Address
		dnsRecordCopy.Status.IPv6Address = dnsRecord.Spec.IPv6Address
		dnsRecordCopy.Status.Txt = dnsRecord.Spec.Txt
		metamachinery.SetStatusCondition(&dnsRecordCopy.Status.Conditions, metav1.Condition{
			Message:            "DNS record update completed successfully",
			Reason:             "DNSRecordUpdateComplete",
			Status:             metav1.ConditionTrue,
			Type:               duckdnsv1alpha1.DNSRecordOrlKorrect.String(),
			LastTransitionTime: metav1.Now(),
		})

		if err := r.Status().Update(ctx, dnsRecordCopy); err != nil {
			logger.Error(err, "Failed to update DNSRecord status")
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"StatusUpdateFailed",
				"Failed to update status for DNSRecord %s: %v",
				dnsRecord.Name,
				err,
			)
			return ctrl.Result{}, fmt.Errorf("failed to update status for DNSRecord, %w", err)
		}

		return ctrl.Result{}, nil
	}

	// Set up a CronJob to update the DNS record periodically depending on the configuration.
	if !metamachinery.IsStatusConditionTrue(dnsRecord.Status.Conditions, duckdnsv1alpha1.DNSCronJobCreated.String()) {
		if err := r.setupCronJob(ctx, dnsRecord); err != nil {
			logger.Error(err, "Failed to set up CronJob for DNSRecord")
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"CronJobSetupFailed",
				"Failed to set up CronJob for DNSRecord %s: %v",
				dnsRecord.Name,
				err,
			)
			return ctrl.Result{}, fmt.Errorf("failed to set up CronJob for DNSRecord, %w", err)
		}
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// processDeletion handles the cleanup of the DNSRecord when it is being deleted.
func (r *DNSRecordReconciler) processDeletion(
	ctx context.Context,
	dnsRecord *duckdnsv1alpha1.DNSRecord,
) (ctrl.Result, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Processing deletion of DNSRecord")
	defer logger.Info("Deletion of DNSRecord processed")

	// Remove the CronJob if it exists.
	cronJob := new(batchv1.CronJob)
	if err := r.Get(
		ctx,
		types.NamespacedName{
			Namespace: dnsRecord.Status.CronJobRef.Namespace,
			Name:      dnsRecord.Status.CronJobRef.Name,
		},
		cronJob,
	); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get CronJob for DNSRecord")
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"CronJobGetFailed",
				"Failed to get CronJob %s/%s for DNSRecord %s: %v",
				dnsRecord.Status.CronJobRef.Namespace,
				dnsRecord.Status.CronJobRef.Name,
				dnsRecord.Name,
				err,
			)
			return ctrl.Result{}, fmt.Errorf(
				"failed to get CronJob %s/%s for DNSRecord %s, %w",
				dnsRecord.Status.CronJobRef.Namespace,
				dnsRecord.Status.CronJobRef.Name,
				dnsRecord.Name,
				err,
			)
		}
	}

	if cronJob != nil && !cronJob.CreationTimestamp.IsZero() && cronJob.DeletionTimestamp.IsZero() {
		logger.Info("Deleting CronJob for DNSRecord", "CronJob", cronJob.Name)
		if err := r.Delete(ctx, cronJob); err != nil {
			logger.Error(err, "Failed to delete CronJob for DNSRecord", "CronJob", cronJob.Name)
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"CronJobDeleteFailed",
				"Failed to delete CronJob %s for DNSRecord %s: %v",
				cronJob.Name,
				dnsRecord.Name,
				err,
			)
			return ctrl.Result{}, fmt.Errorf("failed to delete CronJob %s for DNSRecord %s, %w", cronJob.Name, dnsRecord.Name, err)
		}
		logger.Info("CronJob deleted successfully", "CronJob", cronJob.Name)
		r.Recorder.Eventf(
			dnsRecord,
			"Normal",
			"CronJobDeleted",
			"CronJob %s deleted successfully for DNSRecord %s",
			cronJob.Name,
			dnsRecord.Name,
		)
	}

	// Remove the finalizer from the DNSRecord
	if ctrlutil.ContainsFinalizer(dnsRecord, DNSRecordFinalizer) {
		logger.Info("Removing finalizer from DNSRecord")

		dnsRecordCopy := dnsRecord.DeepCopy()
		if !ctrlutil.RemoveFinalizer(dnsRecordCopy, DNSRecordFinalizer) {
			logger.Error(nil, "Failed to remove finalizer from DNSRecord")
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"FinalizerRemoveFailed",
				"Failed to remove finalizer %s from DNSRecord %s",
				DNSRecordFinalizer,
				dnsRecord.Name,
			)
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer %s", DNSRecordFinalizer)
		}

		if err := r.Update(ctx, dnsRecordCopy); err != nil {
			logger.Error(err, "Failed to remove finalizer", "finalizer", DNSRecordFinalizer)
			return ctrl.Result{}, fmt.Errorf("failed to remove finalizer %s", DNSRecordFinalizer)
		}

		r.Recorder.Eventf(
			dnsRecord,
			"Normal",
			"FinalizerRemoved",
			"Finalizer %s removed from DNSRecord %s",
			DNSRecordFinalizer,
			dnsRecord.Name,
		)
	}

	return ctrl.Result{}, nil
}

// setupCronJob creates a CronJob to update the DNS record periodically.
func (r *DNSRecordReconciler) setupCronJob(ctx context.Context, dnsRecord *duckdnsv1alpha1.DNSRecord) error {
	logger := logf.FromContext(ctx)
	logger.Info("Setting up CronJob for DNSRecord")
	defer logger.Info("CronJob setup completed for DNSRecord")

	cronJobName := fmt.Sprintf("%s-%s-cronjob", dnsRecord.Namespace, dnsRecord.Name)
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: dnsRecord.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   dnsRecord.Spec.CronJob.Schedule,
			Suspend:                    proto.Bool(true),
			FailedJobsHistoryLimit:     proto.Int32(2),
			SuccessfulJobsHistoryLimit: proto.Int32(2),
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cronJobName,
					Namespace: dnsRecord.Namespace,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:      cronJobName,
							Namespace: dnsRecord.Namespace,
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:    "duckdns-updater",
									Image:   dnsRecord.Spec.CronJob.Image,
									Command: dnsRecord.Spec.CronJob.Command,
								},
							},
						},
					},
				},
			},
		},
	}

	if err := r.Create(ctx, cronJob); err != nil {
		logger.Error(err, "Failed to create CronJob for DNSRecord", "CronJob", cronJobName)
		r.Recorder.Eventf(
			dnsRecord,
			"Warning",
			"CronJobCreationFailed",
			"Failed to create CronJob %s for DNSRecord %s: %v",
			cronJobName,
			dnsRecord.Name,
			err,
		)
		return fmt.Errorf("failed to create CronJob %s for DNSRecord %s, %w", cronJobName, dnsRecord.Name, err)
	}

	logger.Info("CronJob created successfully", "CronJob", cronJobName)
	r.Recorder.Eventf(
		dnsRecord,
		"Normal",
		"CronJobCreated",
		"CronJob %s created successfully",
		cronJobName,
	)

	dnsRecordCopy := dnsRecord.DeepCopy()
	cronJobRef := client.ObjectKeyFromObject(cronJob)
	dnsRecordCopy.Status.CronJobRef = duckdnsv1alpha1.CronJobRef{
		Namespace: cronJobRef.Namespace,
		Name:      cronJobRef.Name,
	}
	metamachinery.SetStatusCondition(&dnsRecordCopy.Status.Conditions, metav1.Condition{
		Message:            "CronJob created successfully",
		Reason:             "DNSCronJobCreated",
		Status:             metav1.ConditionTrue,
		Type:               duckdnsv1alpha1.DNSCronJobCreated.String(),
		LastTransitionTime: metav1.Now(),
	})

	if err := r.Status().Update(ctx, dnsRecordCopy); err != nil {
		logger.Error(err, "Failed to update DNSRecord status with CronJob creation")
		r.Recorder.Eventf(
			dnsRecord,
			"Warning",
			"CronJobStatusUpdateFailed",
			"Failed to update status for DNSRecord %s with CronJob creation: %v",
			dnsRecord.Name,
			err,
		)
		return fmt.Errorf("failed to update status for DNSRecord with CronJob creation, %w", err)
	}

	return nil
}

// domainsToString converts a slice of domain names to a comma-separated string.
func (r *DNSRecordReconciler) domainsToString(domains []string) string {
	if len(domains) == 0 {
		return ""
	}
	result := ""
	for _, domain := range domains {
		if result != "" {
			result += ","
		}
		result += domain
	}
	return result
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&duckdnsv1alpha1.DNSRecord{}).
		Named(r.ControllerName).
		Complete(r)
}
