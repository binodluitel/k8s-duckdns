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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	duckdnsv1alpha1 "github.com/binodluitel/k8s-duckdns/api/v1alpha1"
	testutils "github.com/binodluitel/k8s-duckdns/test/utils"
)

var _ = Describe("DNSRecord Controller", func() {
	Context("When reconciling a resource", func() {
		ctx := context.Background()

		dnsRecordName := "test-dns-record"
		namespace := testutils.Namespace{}
		namespacedName := types.NamespacedName{}
		dnsRecord := &duckdnsv1alpha1.DNSRecord{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind DNSRecord")
			ns, err := namespace.Create(ctx, k8sClient)
			Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")
			namespacedName = types.NamespacedName{Name: dnsRecordName, Namespace: ns.Name}
			if err := k8sClient.Get(ctx, namespacedName, dnsRecord); err != nil && errors.IsNotFound(err) {
				dnsRecord = &duckdnsv1alpha1.DNSRecord{
					ObjectMeta: metav1.ObjectMeta{
						Name:      namespacedName.Name,
						Namespace: namespacedName.Namespace,
					},
					Spec: duckdnsv1alpha1.DNSRecordSpec{
						Domains: []string{"example.com"},
					},
				}
				Expect(k8sClient.Create(ctx, dnsRecord)).To(Succeed())
			}
		})

		AfterEach(func() {
			dnsRecord = &duckdnsv1alpha1.DNSRecord{}
			err := k8sClient.Get(ctx, namespacedName, dnsRecord)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance DNSRecord")
			Expect(k8sClient.Delete(ctx, dnsRecord)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			dnsRecordReconciler := &DNSRecordReconciler{
				Client:   k8sClient,
				Scheme:   k8sClient.Scheme(),
				Recorder: testutils.NewFakeRecorder(10, false),
			}

			_, err := dnsRecordReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: namespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})
