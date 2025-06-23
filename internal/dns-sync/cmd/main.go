package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"

	duckdnsv1alpha1 "github.com/binodluitel/k8s-duckdns/api/v1alpha1"
	"github.com/binodluitel/k8s-duckdns/internal/dns-sync/clients/duckdns"
	"github.com/binodluitel/k8s-duckdns/internal/dns-sync/config"
	corev1 "k8s.io/api/core/v1"
	metamachinery "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	logger = ctrl.Log.WithName("dns.synchronizer")
)

func main() {
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()
	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	logger.Info("Starting DNS Synchronizer")
	cfg, err := config.Get()
	if err != nil {
		logger.Error(err, "Failed to get configuration")
		os.Exit(1)
	}

	kcfg, err := ctrl.GetConfig()
	if err != nil {
		logger.Error(err, "unable to get kubeconfig")
		os.Exit(1)
	}

	err = duckdnsv1alpha1.AddToScheme(scheme.Scheme)
	k8sClient, err := client.New(kcfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		logger.Error(err, "unable to create Kubernetes client")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	dnsRecord := new(duckdnsv1alpha1.DNSRecord)
	if err := k8sClient.Get(
		ctx,
		client.ObjectKey{Name: cfg.DNSRecord.Name, Namespace: cfg.DNSRecord.Namespace},
		dnsRecord,
	); err != nil {
		logger.Error(err, "Failed to get DNSRecord resource")
		os.Exit(1)
	}

	ipv4 := dnsRecord.Spec.IPv4Address
	ipv6 := dnsRecord.Spec.IPv6Address
	if ipv4 == "" {
		if changed, changedIPv4 := ipv4Address(ipv4); changed {
			ipv4 = changedIPv4
		}
	}
	if dnsRecord.Spec.AutoUpdateIPv6Address {
		if changed, changedIPv6 := ipv6Address(ipv6); changed {
			ipv6 = changedIPv6
		}
	}

	tokenSecret, err := duckDNSTokenSecret(ctx, k8sClient, dnsRecord)
	if err != nil {
		logger.Error(err, "Failed to retrieve DuckDNS token secret")
		os.Exit(1)
	}

	var clearDNSRecord bool
	annotations := dnsRecord.GetAnnotations()
	if annotateClear, ok := annotations[duckdnsv1alpha1.AnnotationClear]; ok {
		clearDNSRecord, err = strconv.ParseBool(annotateClear)
		if err != nil {
			logger.Error(err, "Failed to parse clear annotation", "value", annotateClear)
			os.Exit(1)
		}
	}

	duckDNSClient, err := duckdns.NewClient(
		cfg.DuckDNS.Protocol,
		cfg.DuckDNS.Domain,
		duckdns.EnableVerbosity(cfg.DuckDNS.Verbose))
	if err != nil {
		logger.Error(err, "Failed to create DuckDNS client")
		os.Exit(1)
	}

	updateURL, err := duckDNSUpdateURL(
		duckDNSClient,
		string(tokenSecret.Data[dnsRecord.Spec.SecretRef.Key]),
		domainsToString(dnsRecord.Spec.Domains),
		ipv4,
		ipv6,
		dnsRecord.Spec.Txt,
		clearDNSRecord,
	)
	if err != nil {
		logger.Error(err, "Failed to generate DuckDNS update URL")
		os.Exit(1)
	}
	if err := redactAndPrint(updateURL); err != nil {
		logger.Error(err, "Failed to redact and print update URL")
		os.Exit(1)
	}

	// Execute the DuckDNS update request
	resp, err := duckDNSClient.ExecuteUpdateRequest(updateURL)
	if err != nil {
		logger.Error(err, "Failed to execute DuckDNS update request")
		os.Exit(1)
	}
	logger.Info("DuckDNS update response", "response", resp)

	// Update the DNSRecord status with the new IPv4 and IPv6 addresses
	// Reset annotations to avoid re-triggering the clear on next sync
	dnsRecordCopy := dnsRecord.DeepCopy()
	dnsRecordCopy.Status.IPv4Address = ipv4
	dnsRecordCopy.Status.IPv6Address = ipv6
	dnsRecord.Status.Txt = dnsRecord.Spec.Txt
	metamachinery.SetStatusCondition(&dnsRecordCopy.Status.Conditions, metav1.Condition{
		Message:            "DNS record updated successfully",
		Reason:             "DNSRecordUpdated",
		Status:             metav1.ConditionTrue,
		Type:               duckdnsv1alpha1.DNSRecordOrlKorrect.String(),
		LastTransitionTime: metav1.Now(),
	})
	if clearDNSRecord {
		dnsRecordCopy.Annotations[duckdnsv1alpha1.AnnotationClear] = "false"
	}
	if err := k8sClient.Status().Update(ctx, dnsRecordCopy); err != nil {
		logger.Error(err, "Failed to update DNSRecord status")
		os.Exit(1)
	}
	logger.Info("DNSRecord status updated successfully", "IPv4", ipv4, "IPv6", ipv6)
	os.Exit(0)
}

func ipv4Address(currentIPv4 string) (bool, string) {
	logger.Info("Current public IPv4 address", "address", currentIPv4)
	updatedIPv4 := currentIPv4
	logger.Info("Updated public IPv4 address", "address", updatedIPv4)
	return false, currentIPv4
}

func ipv6Address(currentIPv6 string) (bool, string) {
	logger.Info("Current public IPv6 address", "address", currentIPv6)
	updatedIPv6 := currentIPv6
	logger.Info("Updated public IPv4 address", "address", updatedIPv6)
	return false, currentIPv6
}

// duckDNSTokenSecret retrieves the secret containing the DuckDNS token for the given DNSRecord.
func duckDNSTokenSecret(
	ctx context.Context,
	k8sClient client.Client,
	dnsRecord *duckdnsv1alpha1.DNSRecord,
) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	secretKey := types.NamespacedName{
		Name:      dnsRecord.Spec.SecretRef.Name,
		Namespace: dnsRecord.Spec.SecretRef.Namespace,
	}
	if err := k8sClient.Get(ctx, secretKey, secret); err != nil {
		return nil, fmt.Errorf(
			"failed to get Secret %s for DNSRecord %s, %w",
			secretKey.String(),
			dnsRecord.Name,
			err,
		)
	}
	return secret, nil
}

// duckDNSUpdateURL generates an update URL for DuckDNS based on the provided DNSRecord specification.
func duckDNSUpdateURL(
	client *duckdns.Client,
	token,
	domains,
	ipv4,
	ipv6,
	txt string,
	clear bool,
) (string, error) {
	duckDNSUpdateURL, err := client.UpdateURL(duckdns.Params{
		Domains: domains,
		IPv4:    ipv4,
		IPv6:    ipv6,
		Clear:   clear,
		Txt:     txt,
		Token:   token,
	})
	if err != nil {
		return "", fmt.Errorf("failed to get duckdns update URL: %w", err)
	}
	return duckDNSUpdateURL, nil
}

// domainsToString converts a slice of domain names to a comma-separated string.
func domainsToString(domains []string) string {
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

func redactAndPrint(updateURL string) error {
	parsedURL, err := url.Parse(updateURL)
	if err != nil {
		return fmt.Errorf("failed to parse update URL: %w", err)
	}
	query := parsedURL.Query()
	query.Set("token", "REDACTED")
	parsedURL.RawQuery = query.Encode()
	logger.V(1).Info("Generated update URL for DuckDNS", "URL", parsedURL.String())
	return nil
}
