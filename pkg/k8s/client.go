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
// Only provides host + CA — caller sets the bearer token.
func baseConfig() (*rest.Config, error) {
	// Standard in-cluster path
	if _, err := os.Stat("/var/run/secrets/kubernetes.io/serviceaccount/token"); err == nil {
		cfg, err := rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("in-cluster config: %w", err)
		}
		cfg.BearerToken = ""
		cfg.BearerTokenFile = ""
		return cfg, nil
	}

	// Projected SA token path (automountServiceAccountToken: false)
	caPath := os.Getenv("KUBE_SA_CA_PATH")
	if caPath != "" {
		host := os.Getenv("KUBERNETES_SERVICE_HOST")
		port := os.Getenv("KUBERNETES_SERVICE_PORT")
		if host != "" && port != "" {
			return &rest.Config{
				Host:            "https://" + host + ":" + port,
				TLSClientConfig: rest.TLSClientConfig{CAFile: caPath},
			}, nil
		}
	}

	// Out-of-cluster
	host := os.Getenv("KUBERNETES_API_URL")
	if host == "" {
		host = "https://kubernetes.default.svc"
	}
	cfg := &rest.Config{Host: host}

	if caFile := os.Getenv("KUBERNETES_CA_FILE"); caFile != "" {
		cfg.TLSClientConfig = rest.TLSClientConfig{CAFile: caFile}
	} else if os.Getenv("DEV_MODE") == "true" {
		cfg.TLSClientConfig = rest.TLSClientConfig{Insecure: true}
	} else {
		return nil, fmt.Errorf("KUBERNETES_CA_FILE required for out-of-cluster mode (set DEV_MODE=true to bypass)")
	}
	return cfg, nil
}
