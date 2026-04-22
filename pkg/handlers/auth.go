package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	authenticationv1 "k8s.io/api/authentication/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type UserInfo struct {
	Username string   `json:"username"`
	Groups   []string `json:"groups"`
	IsAdmin  bool     `json:"is_admin"`
}

type contextKey string

const userContextKey contextKey = "user"

type cachedUser struct {
	info      *UserInfo
	expiresAt time.Time
}

var (
	authCache   sync.Map
	authClient  *kubernetes.Clientset
	authInitErr error
	authOnce    sync.Once
	devMode     = os.Getenv("DEV_MODE") == "true"
)

func initAuthClient() {
	authOnce.Do(func() {
		var config *rest.Config
		var err error

		tokenPath := os.Getenv("KUBE_SA_TOKEN_PATH")
		caPath := os.Getenv("KUBE_SA_CA_PATH")
		if tokenPath != "" && caPath != "" {
			host := os.Getenv("KUBERNETES_SERVICE_HOST")
			port := os.Getenv("KUBERNETES_SERVICE_PORT")
			if host == "" || port == "" {
				authInitErr = fmt.Errorf("KUBERNETES_SERVICE_HOST/PORT not set")
				slog.Warn("auth client not available", "error", authInitErr)
				return
			}
			config = &rest.Config{
				Host:            "https://" + host + ":" + port,
				BearerTokenFile: tokenPath,
				TLSClientConfig: rest.TLSClientConfig{CAFile: caPath},
			}
		} else {
			config, err = rest.InClusterConfig()
			if err != nil {
				authInitErr = err
				slog.Warn("auth client not available", "error", err)
				return
			}
		}
		authClient, err = kubernetes.NewForConfig(config)
		if err != nil {
			authInitErr = err
			slog.Warn("auth client not available", "error", err)
		}
	})
}

func GetUser(r *http.Request) *UserInfo {
	if user, ok := r.Context().Value(userContextKey).(*UserInfo); ok {
		return user
	}
	return &UserInfo{Username: "", IsAdmin: false}
}

func AuthMiddleware(next http.Handler) http.Handler {
	initAuthClient()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/health") {
			next.ServeHTTP(w, r)
			return
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			if authClient == nil && devMode {
				ctx := context.WithValue(r.Context(), userContextKey, &UserInfo{Username: "anonymous", IsAdmin: true})
				r.Header.Set("X-User-Token", "")
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			slog.Warn("AUDIT: authentication failed", "remote_addr", r.RemoteAddr, "reason", "missing_token")
			httpError(w, http.StatusUnauthorized, "authorization required")
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")

		hash := sha256.Sum256([]byte(token))
		cacheKey := hex.EncodeToString(hash[:])

		if cached, ok := authCache.Load(cacheKey); ok {
			cu := cached.(*cachedUser)
			if time.Now().Before(cu.expiresAt) {
				ctx := context.WithValue(r.Context(), userContextKey, cu.info)
				r.Header.Set("X-User-Token", token)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			authCache.Delete(cacheKey)
		}

		if authClient == nil {
			if devMode {
				ctx := context.WithValue(r.Context(), userContextKey, &UserInfo{Username: "anonymous", IsAdmin: true})
				r.Header.Set("X-User-Token", token)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
			httpError(w, http.StatusServiceUnavailable, "authentication service unavailable")
			return
		}

		tr, err := authClient.AuthenticationV1().TokenReviews().Create(r.Context(), &authenticationv1.TokenReview{
			Spec: authenticationv1.TokenReviewSpec{Token: token},
		}, metav1.CreateOptions{})
		if err != nil || !tr.Status.Authenticated {
			slog.Warn("AUDIT: authentication failed", "remote_addr", r.RemoteAddr, "reason", "invalid_token")
			httpError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		username := tr.Status.User.Username
		groups := tr.Status.User.Groups

		sar, err := authClient.AuthorizationV1().SubjectAccessReviews().Create(r.Context(), &authorizationv1.SubjectAccessReview{
			Spec: authorizationv1.SubjectAccessReviewSpec{
				User:   username,
				Groups: groups,
				ResourceAttributes: &authorizationv1.ResourceAttributes{
					Group:    "c2o.openshift.io",
					Resource: "agents",
					Verb:     "admin",
				},
			},
		}, metav1.CreateOptions{})
		isAdmin := err == nil && sar.Status.Allowed

		user := &UserInfo{
			Username: username,
			Groups:   groups,
			IsAdmin:  isAdmin,
		}

		authCache.Store(cacheKey, &cachedUser{
			info:      user,
			expiresAt: time.Now().Add(60 * time.Second),
		})

		ctx := context.WithValue(r.Context(), userContextKey, user)
		r.Header.Set("X-User-Token", token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// authorizeResource returns true if the user is the owner or an admin.
// If not authorized, it writes a 403 response and returns false.
func authorizeResource(w http.ResponseWriter, user *UserInfo, resourceOwner string) bool {
	if user.IsAdmin || user.Username == resourceOwner {
		return true
	}
	httpError(w, http.StatusForbidden, "access denied")
	return false
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func httpError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}
