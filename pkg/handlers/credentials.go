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

type CreateCredentialsRequest struct {
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Type      string            `json:"type"` // apikey, gcpjson, custom
	Data      map[string]string `json:"data"`
}

type CredentialInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Type      string `json:"type"`
	CreatedAt string `json:"createdAt"`
}

// CreateCredentials creates a Secret in the target namespace for agent credentials.
func CreateCredentials(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")

	var req CreateCredentialsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Namespace == "" || !isValidNamespace(req.Namespace) {
		httpError(w, http.StatusBadRequest, "invalid namespace name")
		return
	}
	if req.Name == "" || !isValidSecretName(req.Name) {
		httpError(w, http.StatusBadRequest, "invalid secret name")
		return
	}
	if len(req.Data) == 0 {
		httpError(w, http.StatusBadRequest, "data is required")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	// Build secret data
	secretData := make(map[string][]byte, len(req.Data))
	for k, v := range req.Data {
		secretData[k] = []byte(v)
	}

	credType := req.Type
	if credType == "" {
		credType = "custom"
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"app":                          "c2o",
				"app.kubernetes.io/managed-by": managedByLabel,
				"c2o.credential-type":          credType,
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: secretData,
	}

	_, err = client.CoreV1().Secrets(req.Namespace).Create(context.Background(), secret, metav1.CreateOptions{})
	if err != nil {
		if isAlreadyExists(err) {
			// Update existing secret
			_, err = client.CoreV1().Secrets(req.Namespace).Update(context.Background(), secret, metav1.UpdateOptions{})
			if err != nil {
				slog.Error("failed to update secret", "error", err)
				httpError(w, http.StatusInternalServerError, "failed to update credentials")
				return
			}
		} else {
			slog.Error("failed to create secret", "error", err)
			httpError(w, http.StatusInternalServerError, "failed to create credentials")
			return
		}
	}

	user := GetUser(r)
	slog.Info("AUDIT: credentials created", "user", user.Username, "name", req.Name, "namespace", req.Namespace, "type", credType, "remote_addr", r.RemoteAddr)
	jsonResponse(w, map[string]string{
		"status": "created",
		"name":   req.Name,
	})
}

// ListCredentials returns c2o credential secrets in a namespace.
func ListCredentials(w http.ResponseWriter, r *http.Request) {
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

	secrets, err := client.CoreV1().Secrets(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/managed-by=c2o-plugin",
	})
	if err != nil {
		slog.Error("failed to list secrets", "error", err)
		httpError(w, http.StatusForbidden, "failed to list credentials")
		return
	}

	creds := make([]CredentialInfo, 0, len(secrets.Items))
	for _, s := range secrets.Items {
		creds = append(creds, CredentialInfo{
			Name:      s.Name,
			Namespace: namespace,
			Type:      s.Labels["c2o.credential-type"],
			CreatedAt: s.CreationTimestamp.Format("2006-01-02T15:04:05Z"),
		})
	}

	jsonResponse(w, creds)
}
