// Package k8sclient provides a thin, in-cluster Kubernetes client scoped to
// the two queries the profiler agent needs:
//
//   - the Node the agent pod is running on
//   - all Pods scheduled to that Node
//
// It is intentionally narrow — no watch/informer machinery, no generic
// dynamic client — because those belong in a future layer once we need them.
package k8sclient

import (
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Client wraps the upstream kubernetes.Interface with the small set of
// operations the profiler agent needs.
type Client struct {
	kc       kubernetes.Interface
	nodeName string
}

// New builds a Client using the in-cluster service-account credentials that
// Kubernetes injects into every pod automatically.
//
// nodeName is read from the NODE_NAME environment variable, which the
// DaemonSet manifest must expose via the Downward API:
//
//	env:
//	  - name: NODE_NAME
//	    valueFrom:
//	      fieldRef:
//	        fieldPath: spec.nodeName
func New() (*Client, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("k8sclient: loading in-cluster config: %w", err)
	}

	kc, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8sclient: building clientset: %w", err)
	}

	nodeName := os.Getenv("NODE_NAME")
	if nodeName == "" {
		return nil, fmt.Errorf("k8sclient: NODE_NAME env var is not set; " +
			"expose it via the Downward API in your DaemonSet manifest")
	}

	return &Client{kc: kc, nodeName: nodeName}, nil
}

// NewWithClientset is an escape hatch for testing — it accepts a pre-built
// kubernetes.Interface (e.g. a fake) and an explicit node name instead of
// reading from the environment.
func NewWithClientset(kc kubernetes.Interface, nodeName string) (*Client, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("k8sclient: nodeName must not be empty")
	}
	return &Client{kc: kc, nodeName: nodeName}, nil
}

// NodeName returns the name of the node this agent is running on.
func (c *Client) NodeName() string {
	return c.nodeName
}

// Node fetches the full Node object for the node this agent is running on.
func (c *Client) Node(ctx context.Context) (*corev1.Node, error) {
	node, err := c.kc.CoreV1().Nodes().Get(ctx, c.nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("k8sclient: getting node %q: %w", c.nodeName, err)
	}
	return node, nil
}

// PodsOnNode returns all Pods currently scheduled to the agent's node,
// across all namespaces.
//
// The list is fetched fresh on every call (no caching). Callers that need
// a live watch should use an informer on top of this client.
func (c *Client) PodsOnNode(ctx context.Context) ([]corev1.Pod, error) {
	list, err := c.kc.CoreV1().Pods("" /* all namespaces */).List(ctx, metav1.ListOptions{
		FieldSelector: "spec.nodeName=" + c.nodeName,
	})
	if err != nil {
		return nil, fmt.Errorf("k8sclient: listing pods on node %q: %w", c.nodeName, err)
	}
	return list.Items, nil
}
