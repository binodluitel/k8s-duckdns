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

	duckdnsv1alpha1 "github.com/binodluitel/k8s-duckdns/api/v1alpha1"
	batchv1 "k8s.io/api/batch/v1"
	metamachinery "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlutil "sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// DNSRecordReconciler reconciles a DNSRecord object
type DNSRecordReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	ControllerName string
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
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=create;delete
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

	if err := r.createCronJobServiceAccount(ctx, dnsRecord); err != nil {
		return fmt.Errorf("failed to create CronJob ServiceAccount, %w", err)
	}

	if err := r.createCronJobRole(ctx, dnsRecord); err != nil {
		return fmt.Errorf("failed to create CronJob Role, %w", err)
	}

	if err := r.createCronJobRoleBinding(ctx, dnsRecord); err != nil {
		return fmt.Errorf("failed to create CronJob RoleBinding, %w", err)
	}

	cron, err := r.createCronJob(ctx, dnsRecord)
	if err != nil {
		return fmt.Errorf("failed to create CronJob for DNSRecord %s, %w", dnsRecord.Name, err)
	}

	dnsRecordCopy := dnsRecord.DeepCopy()
	cronJobRef := client.ObjectKeyFromObject(cron)
	dnsRecordCopy.Status.CronJobRef = duckdnsv1alpha1.CronJobRef{
		Namespace: cronJobRef.Namespace,
		Name:      cronJobRef.Name,
	}
	if dnsRecordCopy.Status.Conditions == nil {
		dnsRecordCopy.Status.Conditions = []metav1.Condition{}
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

	logger.Info("CronJob reference added to DNS record successfully", "CronJob", cron.GetName())

	return nil
}

func (r *DNSRecordReconciler) createCronJobServiceAccount(
	ctx context.Context,
	dnsRecord *duckdnsv1alpha1.DNSRecord,
) error {
	logger := logf.FromContext(ctx)
	logger.Info("Creating service account for CronJob of a DNSRecord")
	defer logger.Info("Service account creation for CronJob is completed")

	serviceAccount := cronJobServiceAccount(dnsRecord)
	if err := r.Create(ctx, serviceAccount); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			logger.Error(
				err,
				"Failed to create CronJob ServiceAccount for DNSRecord",
				"ServiceAccount",
				serviceAccount.GetName(),
			)
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"CronJobServiceAccountCreationFailed",
				"Failed to create CronJob ServiceAccount %s for DNSRecord %s: %v",
				serviceAccount.GetName(),
				dnsRecord.Name,
				err,
			)
			return fmt.Errorf("failed to create CronJob ServiceAccount, %w", err)
		}
		r.Recorder.Eventf(
			dnsRecord,
			"Normal",
			"CronJobServiceAccountAlreadyExists",
			"CronJob ServiceAccount %s already exists",
			serviceAccount.GetName(),
		)
		return nil
	}
	r.Recorder.Eventf(
		dnsRecord,
		"Normal",
		"CronJobServiceAccountCreated",
		"CronJob ServiceAccount %s created successfully",
		serviceAccount.GetName(),
	)
	logger.Info("Service account created successfully", "ServiceAccount", serviceAccount.GetName())
	return nil
}

func (r *DNSRecordReconciler) createCronJobRole(
	ctx context.Context,
	dnsRecord *duckdnsv1alpha1.DNSRecord,
) error {
	logger := logf.FromContext(ctx)
	logger.Info("Creating role for CronJob of a DNSRecord")
	defer logger.Info("Role creation for CronJob is completed")

	role := cronJobRole(dnsRecord)
	if err := r.Create(ctx, role); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			logger.Error(
				err,
				"Failed to create CronJob Role for DNSRecord",
				"Role",
				role.GetName(),
			)
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"CronJobRoleCreationFailed",
				"Failed to create CronJob Role %s for DNSRecord %s: %v",
				role.GetName(),
				dnsRecord.Name,
				err,
			)
			return fmt.Errorf("failed to create CronJob Role, %w", err)
		}
		r.Recorder.Eventf(
			dnsRecord,
			"Normal",
			"CronJobRoleAlreadyExists",
			"CronJob Role %s already exists",
			role.GetName(),
		)
		return nil
	}
	r.Recorder.Eventf(
		dnsRecord,
		"Normal",
		"CronJobRoleCreated",
		"CronJob Role %s created successfully",
		role.GetName(),
	)
	logger.Info("Role created successfully", "Role", role.GetName())
	return nil
}

func (r *DNSRecordReconciler) createCronJobRoleBinding(
	ctx context.Context,
	dnsRecord *duckdnsv1alpha1.DNSRecord,
) error {
	logger := logf.FromContext(ctx)
	logger.Info("Creating role binding for CronJob of a DNSRecord")
	defer logger.Info("Role binding creation for CronJob is completed")

	roleBinding := cronJobRoleBinding(dnsRecord)
	if err := r.Create(ctx, roleBinding); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			logger.Error(
				err,
				"Failed to create CronJob RoleBinding for DNSRecord",
				"RoleBinding",
				roleBinding.GetName(),
			)
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"CronJobRoleBindingCreationFailed",
				"Failed to create CronJob RoleBinding %s for DNSRecord %s: %v",
				roleBinding.GetName(),
				dnsRecord.Name,
				err,
			)
			return fmt.Errorf("failed to create CronJob RoleBinding, %w", err)
		}
		r.Recorder.Eventf(
			dnsRecord,
			"Normal",
			"CronJobRoleBindingAlreadyExists",
			"CronJob RoleBinding %s already exists",
			roleBinding.GetName(),
		)
		return nil
	}
	r.Recorder.Eventf(
		dnsRecord,
		"Normal",
		"CronJobRoleBindingCreated",
		"CronJob RoleBinding %s created successfully",
		roleBinding.GetName(),
	)
	logger.Info("Role binding created successfully", "RoleBinding", roleBinding.GetName())
	return nil
}

func (r *DNSRecordReconciler) createCronJob(
	ctx context.Context,
	dnsRecord *duckdnsv1alpha1.DNSRecord,
) (*batchv1.CronJob, error) {
	logger := logf.FromContext(ctx)
	logger.Info("Creating CronJob for DNSRecord")
	defer logger.Info("CronJob creation for DNSRecord is completed")

	cron := cronJob(dnsRecord)
	if err := r.Create(ctx, cron); err != nil {
		if client.IgnoreAlreadyExists(err) != nil {
			logger.Error(err, "Failed to create CronJob for DNSRecord", "CronJob", cron.GetName())
			r.Recorder.Eventf(
				dnsRecord,
				"Warning",
				"CronJobCreationFailed",
				"Failed to create CronJob %s for DNSRecord %s: %v",
				cron.GetName(),
				dnsRecord.Name,
				err,
			)
			return nil, fmt.Errorf(
				"failed to create CronJob %s for DNSRecord %s, %w",
				cron.GetName(),
				dnsRecord.Name,
				err,
			)
		}
		r.Recorder.Eventf(
			dnsRecord,
			"Normal",
			"CronJobAlreadyExists",
			"CronJob %s already exists",
			cron.GetName(),
		)
		return cron, nil
	}
	r.Recorder.Eventf(
		dnsRecord,
		"Normal",
		"CronJobCreated",
		"CronJob %s created successfully",
		cron.GetName(),
	)
	logger.Info("CronJob created successfully", "CronJob", cron.GetName())
	return cron, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&duckdnsv1alpha1.DNSRecord{}).
		Named(r.ControllerName).
		Complete(r)
}
