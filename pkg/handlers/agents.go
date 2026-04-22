package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

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
	DeployedBy string `json:"deployedBy"`
	Replicas   int32  `json:"replicas"`
}

// ListAgents returns c2o agent deployments in a namespace.
func ListAgents(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	token := r.Header.Get("X-User-Token")
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" || !isValidNamespace(namespace) {
		httpError(w, http.StatusBadRequest, "invalid namespace parameter")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	labelSelector := "app=c2o,app.kubernetes.io/managed-by=c2o-plugin"
	if r.URL.Query().Get("mine") == "true" && !user.IsAdmin {
		labelSelector += ",c2o.deployed-by=" + sanitizeLabelValue(user.Username)
	}

	deployments, err := client.AppsV1().Deployments(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: labelSelector,
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

		var specReplicas int32 = 1
		if d.Spec.Replicas != nil {
			specReplicas = *d.Spec.Replicas
		}

		status := "Pending"
		ready := false
		if d.Status.ReadyReplicas > 0 {
			status = "Running"
			ready = true
		} else if specReplicas == 0 {
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

		deployedBy := d.Annotations["c2o.openshift.io/deployed-by"]

		agents = append(agents, AgentInfo{
			Name:       d.Name,
			Namespace:  namespace,
			Instance:   instance,
			Status:     status,
			Ready:      ready,
			Image:      image,
			Age:        age,
			AgentType:  agentType,
			DeployedBy: deployedBy,
			Replicas:   specReplicas,
		})
	}

	jsonResponse(w, agents)
}

type ScaleRequest struct {
	Replicas int32 `json:"replicas"`
}

// ScaleAgent sets the replica count for an agent deployment.
func ScaleAgent(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	token := r.Header.Get("X-User-Token")
	name := mux.Vars(r)["name"]
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" || !isValidNamespace(namespace) {
		httpError(w, http.StatusBadRequest, "invalid namespace parameter")
		return
	}

	var req ScaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Replicas < 0 || req.Replicas > 1 {
		httpError(w, http.StatusBadRequest, "replicas must be 0 or 1")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	deployment, err := client.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		httpError(w, http.StatusNotFound, "agent not found")
		return
	}

	owner := deployment.Annotations["c2o.openshift.io/deployed-by"]
	if owner != "" {
		if !authorizeResource(w, user, owner) {
			slog.Warn("AUDIT: scale denied", "user", user.Username, "name", name, "namespace", namespace, "owner", owner, "remote_addr", r.RemoteAddr)
			return
		}
	}

	deployment.Spec.Replicas = &req.Replicas
	_, err = client.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
	if err != nil {
		slog.Error("failed to scale deployment", "error", err, "name", name)
		httpError(w, http.StatusInternalServerError, "failed to scale agent")
		return
	}

	slog.Info("AUDIT: agent scaled", "user", user.Username, "name", name, "namespace", namespace, "replicas", req.Replicas, "remote_addr", r.RemoteAddr)
	jsonResponse(w, map[string]any{"status": "scaled", "name": name, "replicas": req.Replicas})
}

type AddAgentRequest struct {
	Namespace      string `json:"namespace"`
	AgentType      string `json:"agentType"`
	Prefix         string `json:"prefix"`
	CredentialName string `json:"credentialName"`
	Image          string `json:"image"`
}

// AddAgent deploys one new agent instance, auto-detecting the next available instance number.
func AddAgent(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	token := r.Header.Get("X-User-Token")

	var req AddAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Namespace == "" || !isValidNamespace(req.Namespace) {
		httpError(w, http.StatusBadRequest, "invalid namespace")
		return
	}
	if req.Prefix == "" {
		req.Prefix = "agent"
	}
	if req.AgentType == "" {
		req.AgentType = "claude"
	}
	if req.Image == "" {
		req.Image = defaultImage
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	// Find the next available instance number
	deployments, err := client.AppsV1().Deployments(req.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app=c2o,app.kubernetes.io/managed-by=c2o-plugin",
	})
	if err != nil {
		httpError(w, http.StatusForbidden, "failed to list agents")
		return
	}

	maxNum := 0
	for _, d := range deployments.Items {
		inst := d.Labels["c2o.instance"]
		if strings.HasPrefix(inst, req.Prefix) {
			suffix := inst[len(req.Prefix):]
			if n, err := strconv.Atoi(suffix); err == nil && n > maxNum {
				maxNum = n
			}
		}
	}
	nextNum := maxNum + 1

	// Auto-detect credential name from existing agents if not provided
	if req.CredentialName == "" && len(deployments.Items) > 0 {
		ref := deployments.Items[0]
		if len(ref.Spec.Template.Spec.Containers) > 0 {
			for _, ef := range ref.Spec.Template.Spec.Containers[0].EnvFrom {
				if ef.SecretRef != nil && ef.SecretRef.Name != "c2o-env" {
					req.CredentialName = ef.SecretRef.Name
					break
				}
			}
		}
	}

	instance := fmt.Sprintf("%s%d", req.Prefix, nextNum)
	deployName := fmt.Sprintf("c2o-%s", instance)

	labels := map[string]string{
		"app":                          "c2o",
		"c2o.instance":                 instance,
		"c2o.agent-type":               req.AgentType,
		"c2o.deployed-by":              sanitizeLabelValue(user.Username),
		"app.kubernetes.io/managed-by": managedByLabel,
	}
	annotations := map[string]string{
		"c2o.openshift.io/deployed-by": user.Username,
	}

	if err := ensureAgentServiceAccount(client, req.Namespace); err != nil {
		slog.Error("failed to create agent service account", "error", err)
	}

	if err := createPVC(client, req.Namespace, instance, labels, annotations); err != nil {
		slog.Error("failed to create PVC", "error", err, "instance", instance)
	}

	if err := createDeployment(client, req.Namespace, deployName, instance, req.Image, req.CredentialName, labels, annotations); err != nil {
		slog.Error("failed to create deployment", "error", err, "instance", instance)
		httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create agent %s", instance))
		return
	}

	if err := createServices(client, req.Namespace, instance, labels, annotations); err != nil {
		slog.Error("failed to create services", "error", err, "instance", instance)
	}

	dynClient, dynErr := k8s.DynamicClientFromToken(token)
	if dynErr == nil {
		if err := createGrafanaRoute(dynClient, req.Namespace, instance, labels, annotations); err != nil {
			slog.Error("failed to create grafana route", "error", err, "instance", instance)
		}
	}

	slog.Info("AUDIT: agent added", "user", user.Username, "namespace", req.Namespace, "agent", deployName, "remote_addr", r.RemoteAddr)
	jsonResponse(w, map[string]string{"status": "created", "name": deployName, "instance": instance})
}

// DeleteAgent removes a c2o agent deployment and associated resources.
func DeleteAgent(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	token := r.Header.Get("X-User-Token")
	name := mux.Vars(r)["name"]
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" || !isValidNamespace(namespace) {
		httpError(w, http.StatusBadRequest, "invalid namespace parameter")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	deployment, err := client.AppsV1().Deployments(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		slog.Error("failed to get deployment", "error", err, "name", name)
		httpError(w, http.StatusNotFound, "agent not found")
		return
	}

	owner := deployment.Annotations["c2o.openshift.io/deployed-by"]
	if owner != "" {
		if !authorizeResource(w, user, owner) {
			slog.Warn("AUDIT: delete denied", "user", user.Username, "name", name, "namespace", namespace, "owner", owner, "remote_addr", r.RemoteAddr)
			return
		}
	}

	propagation := metav1.DeletePropagationForeground

	err = client.AppsV1().Deployments(namespace).Delete(context.Background(), name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil {
		slog.Error("failed to delete deployment", "error", err, "name", name)
		httpError(w, http.StatusInternalServerError, "failed to delete agent deployment")
		return
	}

	instance := extractInstance(name)

	for _, prefix := range []string{"c2o-anthropic-", "c2o-openai-", "c2o-grafana-"} {
		_ = client.CoreV1().Services(namespace).Delete(context.Background(), prefix+instance, metav1.DeleteOptions{})
	}

	pvcName := fmt.Sprintf("c2o-workspace-%s", instance)
	_ = client.CoreV1().PersistentVolumeClaims(namespace).Delete(context.Background(), pvcName, metav1.DeleteOptions{})

	slog.Info("AUDIT: agent deleted", "user", user.Username, "name", name, "namespace", namespace, "owner", owner, "remote_addr", r.RemoteAddr)
	jsonResponse(w, map[string]string{"status": "deleted", "name": name})
}

func extractInstance(deploymentName string) string {
	if len(deploymentName) > 4 && deploymentName[:4] == "c2o-" {
		return deploymentName[4:]
	}
	return deploymentName
}
