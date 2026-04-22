package k8s

import (
	"fmt"
	"os"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ClientFromToken creates a Kubernetes client using the user's bearer token.
// This ensures all operations respect the user's RBAC permissions.
func ClientFromToken(token string) (*kubernetes.Clientset, error) {
	cfg, err := baseConfig()
	if err != nil {
		return nil, err
	}
	cfg.BearerToken = token
	cfg.BearerTokenFile = ""
	return kubernetes.NewForConfig(cfg)
}

// DynamicClientFromToken creates a dynamic Kubernetes client using the user's bearer token.
func DynamicClientFromToken(token string) (dynamic.Interface, error) {
	cfg, err := baseConfig()
	if err != nil {
		return nil, err
	}
	cfg.BearerToken = token
	cfg.BearerTokenFile = ""
	return dynamic.NewForConfig(cfg)
}

// baseConfig returns the base cluster config (in-cluster or from env).
func baseConfig() (*rest.Config, error) {
	// In-cluster
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("in-cluster config: %w", err)
		}
		// Clear SA token — we'll use user token instead
		cfg.BearerToken = ""
		cfg.BearerTokenFile = ""
		return cfg, nil
	}

	// Out-of-cluster: use API server from env or default
	host := os.Getenv("KUBERNETES_API_URL")
	if host == "" {
		host = "https://kubernetes.default.svc"
	}
	return &rest.Config{
		Host: host,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: true,
		},
	}, nil
}
