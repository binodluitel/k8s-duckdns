package utils

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Namespace is a test utility for creating Kubernetes namespaces.
type Namespace struct {
	IgnoreAlreadyExists bool
}

// NamespaceOptions is a function that modifies a namespace object.
type NamespaceOptions func(*corev1.Namespace)

// Create creates a namespace with a random name with provided options.
func (ns Namespace) Create(
	ctx context.Context,
	k8sClient client.Client,
	opts ...NamespaceOptions,
) (*corev1.Namespace, error) {
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: ns.randomID("test-")},
	}
	for _, opt := range opts {
		opt(namespace)
	}
	if err := k8sClient.Create(ctx, namespace); err != nil {
		if ns.IgnoreAlreadyExists && errors.IsAlreadyExists(err) {
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(namespace), namespace); err != nil {
				return nil, fmt.Errorf("failed to get existing namespace %s: %w", namespace.Name, err)
			}
			return namespace, nil
		}
		return nil, fmt.Errorf("failed to create namespace %s: %w", namespace.Name, err)
	}
	return namespace, nil
}

// WithLabels appends the given labels to the namespace label.
// If provided key(s) already exists, they will be overwritten.
func (ns Namespace) WithLabels(labels map[string]string) NamespaceOptions {
	return func(cns *corev1.Namespace) {
		if cns.Labels == nil {
			cns.Labels = make(map[string]string)
		}
		for k, v := range labels {
			cns.Labels[k] = v
		}
	}
}

// WithName sets the name of the namespace.
func (ns Namespace) WithName(name string) NamespaceOptions {
	return func(cns *corev1.Namespace) {
		cns.ObjectMeta.Name = name
	}
}

// randomID generates a random UUID and returns it as a string.
func (ns Namespace) randomID(prefix ...string) string {
	id := uuid.NewUUID()
	if len(prefix) > 0 {
		return fmt.Sprintf("%s-%s", prefix[0], id)
	}
	return string(id)
}
