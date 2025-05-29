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

	// CronBackoffLimit is the maximum number of retries for a CronJob before it is considered failed.
	CronBackoffLimit = 10
)

// +kubebuilder:rbac:groups=duckdns.luitel.dev,resources=dnsrecords,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=duckdns.luitel.dev,resources=dnsrecords/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=duckdns.luitel.dev,resources=dnsrecords/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create
// +kubebuilder:rbac:groups=batch,resources=cronjobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

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

	// Set up a CronJob to update the DNS record periodically depending on the configuration.
	if dnsRecord.Status.CronJobRef.Namespace == "" || dnsRecord.Status.CronJobRef.Name == "" {
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

	dnsRecordCopy := dnsRecord.DeepCopy()
	dnsRecordCopy.Status.IPv4Address = dnsRecord.Spec.IPv4Address
	dnsRecordCopy.Status.IPv6Address = dnsRecord.Spec.IPv6Address
	dnsRecordCopy.Status.Txt = dnsRecord.Spec.Txt
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
		return ctrl.Result{}, fmt.Errorf("failed to update status for DNSRecord %s, %w", dnsRecord.Name, err)
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

	// Check if the secret exists for the DuckDNS token
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      dnsRecord.Spec.SecretRef.Name,
		Namespace: dnsRecord.Spec.SecretRef.Namespace,
	}
	if err := r.Get(ctx, secretKey, secret); err != nil {
		if client.IgnoreNotFound(err) != nil {
			logger.Error(err, "Failed to get Secret for DNSRecord", "Secret", secretKey)
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"SecretGetFailed",
				"Failed to get Secret %s for DNSRecord %s: %v",
				secretKey.String(),
				dnsRecord.Name,
				err,
			)
			return fmt.Errorf("failed to get Secret %s for DNSRecord %s, %w", secretKey.String(), dnsRecord.Name, err)
		}
		logger.Info("Secret not found, cannot proceed with CronJob setup")
		r.Recorder.Eventf(
			dnsRecord,
			"Warning",
			"SecretNotFound",
			"Secret %s not found for DNSRecord %s, cannot proceed with CronJob setup",
			secretKey.String(),
			dnsRecord.Name,
		)
		return fmt.Errorf("secret %s not found for DNSRecord %s", secretKey.String(), dnsRecord.Name)
	}

	domains := r.domainsToString(dnsRecord.Spec.Domains)
	duckDNSUpdateURL, err := r.DuckDNSClient.UpdateURL(duckdns.Params{
		Domains: domains,
		IPv4:    dnsRecord.Spec.IPv4Address,
		IPv6:    dnsRecord.Spec.IPv6Address,
		Clear:   dnsRecord.Spec.Clear,
		Txt:     dnsRecord.Spec.Txt,
		Token: func(secretData map[string][]byte) string {
			token := string(secretData[dnsRecord.Spec.SecretRef.Key])
			logger.Info("Token found in secret, proceeding with CronJob setup", "token", token)
			return token
		}(secret.Data),
	})

	logger.V(1).Info("Generated update URL for DuckDNS", "URL", duckDNSUpdateURL)

	if err != nil {
		logger.Error(err, "Failed to generate update URL for DuckDNS")
		r.Recorder.Eventf(
			dnsRecord,
			"Warning",
			"UpdateURLGenerationFailed",
			"Failed to generate update URL for DuckDNS: %v",
			err,
		)
		return fmt.Errorf("failed to generate update URL for DuckDNS, %w", err)
	}

	cronJobName := fmt.Sprintf("%s-cronjob", dnsRecord.Name)
	cronJob := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cronJobName,
			Namespace: dnsRecord.Namespace,
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   dnsRecord.Spec.CronJob.Schedule,
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
									Name:  fmt.Sprintf("duckdns-updater-%s", dnsRecord.Name),
									Image: "alpine:3.21.3",
									Command: []string{
										"/bin/sh",
										"-c",
										`apk --no-cache add curl && echo "Updating DuckDNS record for ` + domains + `" && curl -s "` + duckDNSUpdateURL + `"`,
									},
								},
							},
						},
					},
					BackoffLimit: proto.Int32(CronBackoffLimit),
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
