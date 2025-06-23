package controller

import (
	"fmt"

	duckdnsv1alpha1 "github.com/binodluitel/k8s-duckdns/api/v1alpha1"
	"github.com/binodluitel/k8s-duckdns/internal/config"
	"github.com/gogo/protobuf/proto"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func cronJob(dnsRecord *duckdnsv1alpha1.DNSRecord) *batchv1.CronJob {
	cfg := config.MustGet()
	name := dnsRecord.Name
	namespace := dnsRecord.Namespace
	cronJobImage := dnsRecord.Spec.CronJob.Image
	cronJobImagePullPolicy := corev1.PullIfNotPresent
	if dnsRecord.Spec.CronJob.ImagePullPolicy != "" {
		cronJobImagePullPolicy = corev1.PullPolicy(dnsRecord.Spec.CronJob.ImagePullPolicy)
	}
	return &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:            name,
			Namespace:       namespace,
			Labels:          map[string]string{"app.kubernetes.io/name": "k8s-duckdns"},
			OwnerReferences: []metav1.OwnerReference{ownerReference(dnsRecord)},
		},
		Spec: batchv1.CronJobSpec{
			Schedule:                   dnsRecord.Spec.CronJob.Schedule,
			FailedJobsHistoryLimit:     proto.Int32(2),
			SuccessfulJobsHistoryLimit: proto.Int32(2),
			ConcurrencyPolicy:          batchv1.ForbidConcurrent,
			JobTemplate: batchv1.JobTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
						},
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyOnFailure,
							Containers: []corev1.Container{
								{
									Name:            fmt.Sprintf("dns-sync-%s", dnsRecord.Name),
									Image:           cronJobImage,
									ImagePullPolicy: cronJobImagePullPolicy,
									Env: []corev1.EnvVar{
										{
											Name:  "DNSRECORD_NAME",
											Value: dnsRecord.Name,
										},
										{
											Name:  "DNSRECORD_NAMESPACE",
											Value: dnsRecord.Namespace,
										},
										{
											Name:  "DUCKDNS_PROTOCOL",
											Value: cfg.DuckDNS.Protocol,
										},
										{
											Name:  "DUCKDNS_DOMAIN",
											Value: cfg.DuckDNS.Domain,
										},
										{
											Name:  "DUCKDNS_VERBOSE",
											Value: fmt.Sprintf("%t", cfg.DuckDNS.Verbose),
										},
									},
								},
							},
							ServiceAccountName: dnsRecord.Name,
						},
					},
					BackoffLimit: proto.Int32(CronBackoffLimit),
				},
			},
		},
	}
}

func cronJobServiceAccount(dnsRecord *duckdnsv1alpha1.DNSRecord) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dnsRecord.Name,
			Namespace:       dnsRecord.Namespace,
			Labels:          map[string]string{"app.kubernetes.io/name": "k8s-duckdns"},
			OwnerReferences: []metav1.OwnerReference{ownerReference(dnsRecord)},
		},
	}
}

func cronJobRole(dnsRecord *duckdnsv1alpha1.DNSRecord) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dnsRecord.Name,
			Namespace:       dnsRecord.Namespace,
			Labels:          map[string]string{"app.kubernetes.io/name": "k8s-duckdns"},
			OwnerReferences: []metav1.OwnerReference{ownerReference(dnsRecord)},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{"duckdns.luitel.dev"},
				Resources: []string{"dnsrecords"},
				Verbs:     []string{"get", "list", "patch", "update"},
			},
			{
				APIGroups: []string{"duckdns.luitel.dev"},
				Resources: []string{"dnsrecords/status"},
				Verbs:     []string{"get", "patch", "update"},
			},
		},
	}
}

func cronJobRoleBinding(dnsRecord *duckdnsv1alpha1.DNSRecord) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            dnsRecord.Name,
			Namespace:       dnsRecord.Namespace,
			Labels:          map[string]string{"app.kubernetes.io/name": "k8s-duckdns"},
			OwnerReferences: []metav1.OwnerReference{ownerReference(dnsRecord)},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     dnsRecord.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      dnsRecord.Name,
				Namespace: dnsRecord.Namespace,
			},
		},
	}
}

func ownerReference(dnsRecord *duckdnsv1alpha1.DNSRecord) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion:         duckdnsv1alpha1.GroupVersion.String(),
		Kind:               duckdnsv1alpha1.DNSRecordKind,
		Name:               dnsRecord.Name,
		UID:                dnsRecord.UID,
		Controller:         proto.Bool(true),
		BlockOwnerDeletion: proto.Bool(true),
	}
}
