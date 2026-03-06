package parser

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type K8sLogSource struct {
	Namespace  string
	Deployment string
	Kubeconfig string
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return home + path[1:]
	}
	return path
}

func (k *K8sLogSource) ReadLogs() ([]LogEntry, error) {
	config, err := clientcmd.BuildConfigFromFlags("", expandHome(k.Kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ctx := context.Background()

	// Find pods belonging to the deployment via its ReplicaSet labels
	deployment, err := clientset.AppsV1().Deployments(k.Namespace).Get(ctx, k.Deployment, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", k.Namespace, k.Deployment, err)
	}

	selector, err := metav1.LabelSelectorAsSelector(deployment.Spec.Selector)
	if err != nil {
		return nil, fmt.Errorf("failed to parse deployment selector: %w", err)
	}

	pods, err := clientset.CoreV1().Pods(k.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods found for deployment %s/%s", k.Namespace, k.Deployment)
	}

	var allEntries []LogEntry
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		log.Info().Str("pod", pod.Name).Msg("reading logs from pod")
		entries, err := k.readPodLogs(ctx, clientset, pod.Name)
		if err != nil {
			log.Warn().Err(err).Str("pod", pod.Name).Msg("failed to read logs from pod")
			continue
		}
		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

func (k *K8sLogSource) readPodLogs(ctx context.Context, clientset *kubernetes.Clientset, podName string) ([]LogEntry, error) {
	req := clientset.CoreV1().Pods(k.Namespace).GetLogs(podName, &corev1.PodLogOptions{})
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	var entries []LogEntry
	reader := bufio.NewReader(stream)
	for {
		line, err := reader.ReadBytes('\n')
		if len(line) > 0 {
			var entry LogEntry
			if jsonErr := json.Unmarshal(line, &entry); jsonErr != nil {
				continue
			}
			entries = append(entries, entry)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return entries, err
		}
	}

	return entries, nil
}
