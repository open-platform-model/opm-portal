package logs

import (
	"context"
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Source reads Pods and their logs as the reading identity.
type Source interface {
	// Pod returns the Pod namespace/name.
	Pod(ctx context.Context, namespace, name string) (*corev1.Pod, error)
	// Logs opens a container's log. The stream ends when ctx is done or the
	// returned reader is closed.
	Logs(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error)
}

// ClientSource reads through a typed Kubernetes client.
func ClientSource(c kubernetes.Interface) Source { return clientSource{c: c} }

type clientSource struct {
	c kubernetes.Interface
}

func (s clientSource) Pod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	pod, err := s.c.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("getting the pod: %w", err)
	}
	return pod, nil
}

func (s clientSource) Logs(ctx context.Context, namespace, pod string, opts *corev1.PodLogOptions) (io.ReadCloser, error) {
	rc, err := s.c.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("opening the log stream: %w", err)
	}
	return rc, nil
}
