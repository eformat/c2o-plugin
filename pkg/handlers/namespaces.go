package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/rhai-code/c2o-plugin/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type NamespaceInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type CreateNamespaceRequest struct {
	Name string `json:"name"`
}

// ListNamespaces returns namespaces the user has access to.
func ListNamespaces(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")
	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	nsList, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		slog.Error("failed to list namespaces", "error", err)
		httpError(w, http.StatusForbidden, "failed to list namespaces")
		return
	}

	namespaces := make([]NamespaceInfo, 0, len(nsList.Items))
	for _, ns := range nsList.Items {
		namespaces = append(namespaces, NamespaceInfo{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
		})
	}

	jsonResponse(w, namespaces)
}

// CreateNamespace creates a new namespace using the user's token.
func CreateNamespace(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")

	var req CreateNamespaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Name == "" {
		httpError(w, http.StatusBadRequest, "name is required")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: req.Name,
		},
	}

	_, err = client.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
	if err != nil {
		slog.Error("failed to create namespace", "error", err, "name", req.Name)
		httpError(w, http.StatusForbidden, "failed to create namespace: "+err.Error())
		return
	}

	slog.Info("created namespace", "name", req.Name)
	jsonResponse(w, map[string]string{"status": "created", "name": req.Name})
}
