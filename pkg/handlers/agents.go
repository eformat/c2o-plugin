package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rhai-code/c2o-plugin/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AgentInfo struct {
	Name       string `json:"name"`
	Namespace  string `json:"namespace"`
	Instance   string `json:"instance"`
	Status     string `json:"status"`
	Ready      bool   `json:"ready"`
	Image      string `json:"image"`
	Age        string `json:"age"`
	AgentType  string `json:"agentType"`
}

// ListAgents returns c2o agent deployments in a namespace.
func ListAgents(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		httpError(w, http.StatusBadRequest, "namespace parameter required")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	deployments, err := client.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app=c2o,app.kubernetes.io/managed-by=c2o-plugin",
	})
	if err != nil {
		slog.Error("failed to list deployments", "error", err, "namespace", namespace)
		httpError(w, http.StatusForbidden, "failed to list agents in namespace")
		return
	}

	agents := make([]AgentInfo, 0, len(deployments.Items))
	for _, d := range deployments.Items {
		instance := d.Labels["c2o.instance"]
		agentType := d.Labels["c2o.agent-type"]
		if agentType == "" {
			agentType = "claude"
		}

		status := "Pending"
		ready := false
		if d.Status.ReadyReplicas > 0 {
			status = "Running"
			ready = true
		} else if d.Status.Replicas == 0 {
			status = "Scaled Down"
		} else if d.Status.UnavailableReplicas > 0 {
			status = "Unavailable"
		}

		image := ""
		if len(d.Spec.Template.Spec.Containers) > 0 {
			image = d.Spec.Template.Spec.Containers[0].Image
		}

		age := ""
		if !d.CreationTimestamp.IsZero() {
			age = fmt.Sprintf("%s", metav1.Now().Sub(d.CreationTimestamp.Time).Round(1e9).String())
		}

		agents = append(agents, AgentInfo{
			Name:      d.Name,
			Namespace: namespace,
			Instance:  instance,
			Status:    status,
			Ready:     ready,
			Image:     image,
			Age:       age,
			AgentType: agentType,
		})
	}

	jsonResponse(w, agents)
}

// DeleteAgent removes a c2o agent deployment and associated resources.
func DeleteAgent(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")
	name := mux.Vars(r)["name"]
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		httpError(w, http.StatusBadRequest, "namespace parameter required")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	propagation := metav1.DeletePropagationForeground

	// Delete deployment
	err = client.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil {
		slog.Error("failed to delete deployment", "error", err, "name", name)
		httpError(w, http.StatusInternalServerError, "failed to delete agent deployment")
		return
	}

	// Delete associated services
	for _, suffix := range []string{"-anthropic", "-openai"} {
		svcName := name + suffix
		_ = client.CoreV1().Services(namespace).Delete(context.Background(), svcName, metav1.DeleteOptions{})
	}

	// Delete associated PVC
	pvcName := name + "-workspace"
	pvcName = fmt.Sprintf("c2o-workspace-%s", extractInstance(name))
	_ = client.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), pvcName, metav1.DeleteOptions{})

	slog.Info("deleted agent", "name", name, "namespace", namespace)
	jsonResponse(w, map[string]string{"status": "deleted", "name": name})
}

func extractInstance(deploymentName string) string {
	// c2o-agent1 -> agent1
	if len(deploymentName) > 4 && deploymentName[:4] == "c2o-" {
		return deploymentName[4:]
	}
	return deploymentName
}
