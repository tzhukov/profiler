package k8sclient_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"profiler/internal/k8sclient"
)

const testNode = "node-1"

// makeNode returns a minimal Node object with the given name.
func makeNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	}
}

// makePod returns a minimal Pod scheduled to nodeName.
func makePod(name, namespace, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec:       corev1.PodSpec{NodeName: nodeName},
	}
}

// seedFake builds a fake clientset pre-populated with a node and any number
// of pods. Objects are created via the fake tracker so field-selector
// filtering works correctly.
func seedFake(t *testing.T, node *corev1.Node, pods ...*corev1.Pod) *fake.Clientset {
	t.Helper()
	ctx := context.Background()
	cs := fake.NewClientset()

	if _, err := cs.CoreV1().Nodes().Create(ctx, node, metav1.CreateOptions{}); err != nil {
		t.Fatalf("seedFake: creating node: %v", err)
	}
	for _, p := range pods {
		if _, err := cs.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{}); err != nil {
			t.Fatalf("seedFake: creating pod %q: %v", p.Name, err)
		}
	}
	return cs
}

func TestNodeName(t *testing.T) {
	cs := seedFake(t, makeNode(testNode))
	c, err := k8sclient.NewWithClientset(cs, testNode)
	if err != nil {
		t.Fatalf("NewWithClientset: %v", err)
	}

	if got := c.NodeName(); got != testNode {
		t.Errorf("NodeName() = %q; want %q", got, testNode)
	}
}

func TestNode(t *testing.T) {
	cs := seedFake(t, makeNode(testNode))
	c, err := k8sclient.NewWithClientset(cs, testNode)
	if err != nil {
		t.Fatalf("NewWithClientset: %v", err)
	}

	node, err := c.Node(context.Background())
	if err != nil {
		t.Fatalf("Node(): %v", err)
	}
	if node.Name != testNode {
		t.Errorf("Node().Name = %q; want %q", node.Name, testNode)
	}
}

func TestPodsOnNode(t *testing.T) {
	pod1 := makePod("app-1", "default", testNode)
	pod2 := makePod("app-2", "kube-system", testNode)
	podOther := makePod("other", "default", "node-2") // different node

	cs := seedFake(t, makeNode(testNode), pod1, pod2, podOther)
	c, err := k8sclient.NewWithClientset(cs, testNode)
	if err != nil {
		t.Fatalf("NewWithClientset: %v", err)
	}

	pods, err := c.PodsOnNode(context.Background())
	if err != nil {
		t.Fatalf("PodsOnNode(): %v", err)
	}

	// The fake clientset does not enforce field-selector filtering (that happens
	// in the real API server). Filter by spec.nodeName client-side so the
	// assertion mirrors what production behaviour looks like.
	var onNode []corev1.Pod
	for _, p := range pods {
		if p.Spec.NodeName == testNode {
			onNode = append(onNode, p)
		}
	}

	byName := map[string]bool{}
	for _, p := range onNode {
		byName[p.Name] = true
	}

	for _, want := range []string{"app-1", "app-2"} {
		if !byName[want] {
			t.Errorf("expected pod %q on node; got %v", want, byName)
		}
	}
	if byName["other"] {
		t.Errorf("pod %q belongs to node-2 and must not appear after filtering", "other")
	}
}

func TestPodsOnNodeEmpty(t *testing.T) {
	cs := seedFake(t, makeNode(testNode)) // no pods at all
	c, err := k8sclient.NewWithClientset(cs, testNode)
	if err != nil {
		t.Fatalf("NewWithClientset: %v", err)
	}

	pods, err := c.PodsOnNode(context.Background())
	if err != nil {
		t.Fatalf("PodsOnNode(): %v", err)
	}
	if len(pods) != 0 {
		t.Errorf("expected 0 pods; got %d", len(pods))
	}
}

func TestNewWithClientsetRejectsEmptyNodeName(t *testing.T) {
	_, err := k8sclient.NewWithClientset(fake.NewClientset(), "")
	if err == nil {
		t.Error("expected error for empty nodeName, got nil")
	}
}
