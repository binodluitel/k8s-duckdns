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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE: `json` tags are required.
// Any new fields you add must have `json` tags for the fields to be serialized.

// DNSRecordSpec defines the desired state of DNSRecord.
type DNSRecordSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Domains is a list of domains to be managed by the DNSRecord.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Domains []string `json:"domains"`

	// IPv4Address is the IPv4 address to be associated with the domains.
	// If the IP address isn't specified to DuckDNS, then it will be detected -
	// this only works for IPv4 addresses.
	// +kubebuilder:validation:Pattern=`^(\d{1,3}\.){3}\d{1,3}$`
	// +kubebuilder:validation:Optional
	IPv4Address string `json:"ipv4Address,omitempty"`

	// IPv6Address is the IPv6 address to be associated with the domains.
	// You can put either an IPv4 or an IPv6 address in the ip parameter.
	// If the IP address isn't specified to DuckDNS, then it will NOT be detected -
	// and will be left unset.
	// +kubebuilder:validation:Pattern=`^([0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}$`
	// +kubebuilder:validation:Optional
	IPv6Address string `json:"ipv6Address,omitempty"`

	// Clear is a flag to clear the DNS records.
	// If set to true, the DNS records will be cleared for both IPv4 and IPv6 addresses.
	// +kubebuilder:validation:Optional
	Clear bool `json:"clear,omitempty"`

	// Txt is an optional parameter to specify a TXT record to be associated with the domain.
	// +kubebuilder:validation:Optional
	Txt string `json:"txt,omitempty"`

	// CronJob specifications for the DNS record.
	// This cron job will be responsible for updating the DNS record at the specified schedule.
	// +kubebuilder:validation:Required
	CronJob CronJob `json:"cronJob"`
}

// CronJob defines the specifications for a cron job that manages the DNS record.
type CronJob struct {
	// Schedule is the cron schedule for the job.
	// +kubebuilder:validation:Required
	Schedule string `json:"schedule"`

	// Command is the command to be executed by the cron job.
	// +kubebuilder:validation:Required
	Command []string `json:"command"`

	// Image is the container image to be used for the cron job.
	// +kubebuilder:validation:Required
	Image string `json:"image"`
}

// DNSConditionType defines the status condition types for a DNS entry in DuckDNS.
type DNSConditionType string

const (
	// DNSRecordOrlKorrect indicates if the DNS record is all correct, or not.
	DNSRecordOrlKorrect DNSConditionType = "OK"

	// DNSCronJobCreated indicates if the DNS cron job is created successfully.
	DNSCronJobCreated DNSConditionType = "DNSCronJobCreated"
)

// DNSConditionType string representation of DNS condition.
func (c DNSConditionType) String() string {
	return string(c)
}

// DNSRecordStatus defines the observed state of DNSRecord.
type DNSRecordStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define the observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Conditions represent the latest available observations of a DNSRecord's current state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// IPv4Address is the IPv4 address to be associated with the domains.
	IPv4Address string `json:"ipv4Address"`

	// IPv6Address is the IPv6 address to be associated with the domains.
	// +optional
	IPv6Address string `json:"ipv6Address,omitempty"`

	// TXT is the TXT record associated with the domain.
	// +optional
	Txt string `json:"txt,omitempty"`

	// CronJobRef is a reference to the cron job that manages the DNS record.
	CronJobRef CronJobRef `json:"cronJobRef"`
}

// CronJobRef defines a reference to a cron job that manages the DNS record.
type CronJobRef struct {
	// Namespace is the namespace where the cron job is located.
	Namespace string `json:"namespace"`

	// Name is the name of the cron job.
	Name string `json:"name"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// DNSRecord is the Schema for the dnsrecords API.
type DNSRecord struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DNSRecordSpec   `json:"spec,omitempty"`
	Status DNSRecordStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DNSRecordList contains a list of DNSRecord.
type DNSRecordList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSRecord `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DNSRecord{}, &DNSRecordList{})
}
