package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/rhai-code/c2o-plugin/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type NamespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CreateNamespaceRequest struct {
	Name string `json:"name"`
}

var (
	projectGVR        = schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projects"}
	projectRequestGVR = schema.GroupVersionResource{Group: "project.openshift.io", Version: "v1", Resource: "projectrequests"}
)

// ListNamespaces returns projects the user has access to via the OpenShift Project API.
func ListNamespaces(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")
	dynClient, err := k8s.DynamicClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	projectList, err := dynClient.Resource(projectGVR).List(context.Background(), metav1.ListOptions{})
	if err != nil {
		slog.Error("failed to list projects", "error", err)
		httpError(w, http.StatusForbidden, "failed to list projects")
		return
	}

	namespaces := make([]NamespaceInfo, 0, len(projectList.Items))
	for _, p := range projectList.Items {
		status, _, _ := unstructured.NestedString(p.Object, "status", "phase")
		namespaces = append(namespaces, NamespaceInfo{
			Name:   p.GetName(),
			Status: status,
		})
	}

	jsonResponse(w, namespaces)
}

// CreateNamespace creates a new project via OpenShift ProjectRequest API.
func CreateNamespace(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")

	var req CreateNamespaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" || !isValidNamespace(req.Name) {
		httpError(w, http.StatusBadRequest, "invalid namespace name")
		return
	}

	dynClient, err := k8s.DynamicClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	projectRequest := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "project.openshift.io/v1",
			"kind":       "ProjectRequest",
			"metadata": map[string]interface{}{
				"name": req.Name,
			},
		},
	}

	_, err = dynClient.Resource(projectRequestGVR).Create(context.Background(), projectRequest, metav1.CreateOptions{})
	if err != nil {
		slog.Error("failed to create project", "error", err, "name", req.Name)
		httpError(w, http.StatusForbidden, "failed to create project: "+err.Error())
		return
	}

	user := GetUser(r)
	slog.Info("AUDIT: project created", "user", user.Username, "name", req.Name, "remote_addr", r.RemoteAddr)
	jsonResponse(w, map[string]string{"status": "created", "name": req.Name})
}
