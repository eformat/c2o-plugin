package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/rhai-code/c2o-plugin/pkg/k8s"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type DeployRequest struct {
	AgentType      string `json:"agentType"`
	Namespace      string `json:"namespace"`
	Count          int    `json:"count"`
	Prefix         string `json:"prefix"`
	CredentialName string `json:"credentialName"`
	Image          string `json:"image"`
}

type DeployResponse struct {
	Status    string   `json:"status"`
	Namespace string   `json:"namespace"`
	Agents    []string `json:"agents"`
}

const (
	defaultImage    = "quay.io/eformat/c2o:latest"
	managedByLabel  = "c2o-plugin"
)

var invalidLabelChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeLabelValue(s string) string {
	s = invalidLabelChars.ReplaceAllString(s, "-")
	if len(s) > 63 {
		s = s[:63]
	}
	s = strings.Trim(s, "-_.")
	return s
}

// Deploy creates c2o agent instances in the target namespace.
func Deploy(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	token := r.Header.Get("X-User-Token")

	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Namespace == "" || !isValidNamespace(req.Namespace) {
		httpError(w, http.StatusBadRequest, "invalid namespace name")
		return
	}
	if req.Count < 1 || req.Count > 10 {
		httpError(w, http.StatusBadRequest, "count must be between 1 and 10")
		return
	}
	if req.Prefix == "" {
		req.Prefix = "agent"
	} else if !isValidPrefix(req.Prefix) {
		httpError(w, http.StatusBadRequest, "invalid agent prefix")
		return
	}
	if req.AgentType == "" {
		req.AgentType = "claude"
	} else if !isValidAgentType(req.AgentType) {
		httpError(w, http.StatusBadRequest, "invalid agent type")
		return
	}
	if req.Image == "" {
		req.Image = defaultImage
	} else if !isValidImage(req.Image) {
		httpError(w, http.StatusBadRequest, "invalid image reference")
		return
	}
	if req.CredentialName != "" && !isValidSecretName(req.CredentialName) {
		httpError(w, http.StatusBadRequest, "invalid credential name")
		return
	}

	client, err := k8s.ClientFromToken(token)
	if err != nil {
		slog.Error("failed to create k8s client", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create kubernetes client")
		return
	}

	// Ensure namespace exists
	_, err = client.CoreV1().Namespaces().Get(context.Background(), req.Namespace, metav1.GetOptions{})
	if err != nil {
		httpError(w, http.StatusBadRequest, fmt.Sprintf("namespace %q not found or not accessible", req.Namespace))
		return
	}

	// Ensure zero-permission service account for agent pods
	if err := ensureAgentServiceAccount(client, req.Namespace); err != nil {
		slog.Error("failed to create agent service account", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create agent service account")
		return
	}

	// Apply shared configmap
	if err := applyConfigMap(client, req.Namespace); err != nil {
		slog.Error("failed to apply configmap", "error", err)
		httpError(w, http.StatusInternalServerError, "failed to create configmap")
		return
	}

	agentNames := make([]string, 0, req.Count)
	for i := 1; i <= req.Count; i++ {
		instance := fmt.Sprintf("%s%d", req.Prefix, i)
		deployName := fmt.Sprintf("c2o-%s", instance)
		agentNames = append(agentNames, deployName)

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

		// Create PVC
		if err := createPVC(client, req.Namespace, instance, labels, annotations); err != nil {
			slog.Error("failed to create PVC", "error", err, "instance", instance)
		}

		// Create Deployment
		if err := createDeployment(client, req.Namespace, deployName, instance, req.Image, req.CredentialName, labels, annotations); err != nil {
			slog.Error("failed to create deployment", "error", err, "instance", instance)
			httpError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create agent %s", instance))
			return
		}

		// Create Services
		if err := createServices(client, req.Namespace, instance, labels, annotations); err != nil {
			slog.Error("failed to create services", "error", err, "instance", instance)
		}

		// Create Grafana Route
		dynClient, dynErr := k8s.DynamicClientFromToken(token)
		if dynErr != nil {
			slog.Error("failed to create dynamic client for route", "error", dynErr)
		} else {
			if err := createGrafanaRoute(dynClient, req.Namespace, instance, labels, annotations); err != nil {
				slog.Error("failed to create grafana route", "error", err, "instance", instance)
			}
		}
	}

	slog.Info("AUDIT: agents deployed", "user", user.Username, "namespace", req.Namespace, "count", req.Count, "agentType", req.AgentType, "image", req.Image, "agents", agentNames, "remote_addr", r.RemoteAddr)
	jsonResponse(w, DeployResponse{
		Status:    "deployed",
		Namespace: req.Namespace,
		Agents:    agentNames,
	})
}

func createPVC(client *kubernetes.Clientset, namespace, instance string, labels, annotations map[string]string) error {
	pvcName := fmt.Sprintf("c2o-workspace-%s", instance)
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        pvcName,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
	}

	_, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(context.Background(), pvc, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return err
	}
	return nil
}

func createDeployment(client *kubernetes.Clientset, namespace, name, instance, image, credentialName string, labels, annotations map[string]string) error {
	replicas := int32(1)
	pvcName := fmt.Sprintf("c2o-workspace-%s", instance)

	envFrom := []corev1.EnvFromSource{}
	// Always reference c2o-env secret if it exists (shared credentials)
	envFrom = append(envFrom, corev1.EnvFromSource{
		SecretRef: &corev1.SecretEnvSource{
			LocalObjectReference: corev1.LocalObjectReference{Name: "c2o-env"},
			Optional:             boolPtr(true),
		},
	})
	if credentialName != "" && credentialName != "c2o-env" {
		envFrom = append(envFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: credentialName},
			},
		})
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":          "c2o",
					"c2o.instance": instance,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName:           "c2o-agent",
					AutomountServiceAccountToken: boolPtr(false),
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: boolPtr(true),
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{
						{
							Name:  "c2o",
							Image: image,
							Ports: []corev1.ContainerPort{
								{Name: "anthropic", ContainerPort: 8819, Protocol: corev1.ProtocolTCP},
								{Name: "openai", ContainerPort: 8899, Protocol: corev1.ProtocolTCP},
								{Name: "grafana", ContainerPort: 3000, Protocol: corev1.ProtocolTCP},
								{Name: "prometheus", ContainerPort: 9090, Protocol: corev1.ProtocolTCP},
								{Name: "envoy-admin", ContainerPort: 9901, Protocol: corev1.ProtocolTCP},
							},
							EnvFrom: envFrom,
							Env: []corev1.EnvVar{
								{Name: "UPSTREAM_HOST", Value: "localhost"},
								{Name: "ANTHROPIC_BASE_URL", Value: "http://localhost:8819"},
								{Name: "ANTHROPIC_API_KEY", Value: "sk-placeholder"},
								{Name: "KUBECONFIG", Value: "/dev/null"},
							},
							SecurityContext: &corev1.SecurityContext{
								AllowPrivilegeEscalation: boolPtr(false),
								Capabilities: &corev1.Capabilities{
									Drop: []corev1.Capability{"ALL"},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("2Gi"),
									corev1.ResourceCPU:    resource.MustParse("500m"),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceMemory: resource.MustParse("12Gi"),
									corev1.ResourceCPU:    resource.MustParse("4000m"),
								},
							},
							VolumeMounts: gcpVolumeMounts(),
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8819),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								FailureThreshold:    30,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8819),
									},
								},
								PeriodSeconds: 10,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/health",
										Port: intstr.FromInt(8819),
									},
								},
								PeriodSeconds:    30,
								FailureThreshold: 3,
							},
						},
					},
					Volumes: gcpVolumes(pvcName, credentialName),
				},
			},
		},
	}

	_, err := client.AppsV1().Deployments(namespace).Create(context.Background(), deployment, metav1.CreateOptions{})
	if err != nil {
		if isAlreadyExists(err) {
			_, err = client.AppsV1().Deployments(namespace).Update(context.Background(), deployment, metav1.UpdateOptions{})
		}
		return err
	}
	return nil
}

func createServices(client *kubernetes.Clientset, namespace, instance string, labels, annotations map[string]string) error {
	selector := map[string]string{
		"app":          "c2o",
		"c2o.instance": instance,
	}

	services := []struct {
		name     string
		port     int32
		portName string
	}{
		{fmt.Sprintf("c2o-anthropic-%s", instance), 8819, "8819-tcp"},
		{fmt.Sprintf("c2o-openai-%s", instance), 8899, "8899-tcp"},
		{fmt.Sprintf("c2o-grafana-%s", instance), 3000, "3000-tcp"},
	}

	for _, svc := range services {
		service := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:        svc.name,
				Namespace:   namespace,
				Labels:      labels,
				Annotations: annotations,
			},
			Spec: corev1.ServiceSpec{
				Selector: selector,
				Ports: []corev1.ServicePort{
					{
						Name:       svc.portName,
						Port:       svc.port,
						TargetPort: intstr.FromInt(int(svc.port)),
						Protocol:   corev1.ProtocolTCP,
					},
				},
			},
		}

		_, err := client.CoreV1().Services(namespace).Create(context.Background(), service, metav1.CreateOptions{})
		if err != nil && !isAlreadyExists(err) {
			return err
		}
	}
	return nil
}

func applyConfigMap(client *kubernetes.Clientset, namespace string) error {
	// Check if configmap already exists
	_, err := client.CoreV1().ConfigMaps(namespace).Get(context.Background(), "c2o-config", metav1.GetOptions{})
	if err == nil {
		return nil // already exists
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c2o-config",
			Namespace: namespace,
			Labels: map[string]string{
				"app":                          "c2o",
				"app.kubernetes.io/managed-by": managedByLabel,
			},
		},
		Data: map[string]string{
			"CLAUDE_MD": "# c2o Agent\nYou are a c2o coding agent deployed in OpenShift.\n",
		},
	}

	_, err = client.CoreV1().ConfigMaps(namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	return err
}

func ensureAgentServiceAccount(client *kubernetes.Clientset, namespace string) error {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c2o-agent",
			Namespace: namespace,
			Labels: map[string]string{
				"app":                          "c2o",
				"app.kubernetes.io/managed-by": managedByLabel,
			},
		},
		AutomountServiceAccountToken: boolPtr(false),
	}
	_, err := client.CoreV1().ServiceAccounts(namespace).Create(context.Background(), sa, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return err
	}
	return nil
}

func gcpVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{Name: "workspace", MountPath: "/home/user/workspace"},
		{Name: "workspace", MountPath: "/home/user/.claude", SubPath: ".claude"},
		{Name: "workspace", MountPath: "/home/user/.cache", SubPath: ".cache"},
		{Name: "gcp-adc", MountPath: "/home/user/.config/gcloud", ReadOnly: true},
		{Name: "gcp-adc", MountPath: "/adc", ReadOnly: true},
	}
}

func gcpVolumes(pvcName, credentialName string) []corev1.Volume {
	adcSecret := "c2o-env"
	if credentialName != "" {
		adcSecret = credentialName
	}
	return []corev1.Volume{
		{
			Name: "workspace",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: pvcName,
				},
			},
		},
		{
			Name: "gcp-adc",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: adcSecret,
					Optional:   boolPtr(true),
					Items: []corev1.KeyToPath{
						{
							Key:  "GOOGLE_APPLICATION_CREDENTIALS_JSON",
							Path: "application_default_credentials.json",
						},
					},
				},
			},
		},
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func isAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already exists")
}

var routeGVR = schema.GroupVersionResource{
	Group:    "route.openshift.io",
	Version:  "v1",
	Resource: "routes",
}

func createGrafanaRoute(dynClient dynamic.Interface, namespace, instance string, labels, annotations map[string]string) error {
	routeName := fmt.Sprintf("c2o-grafana-%s", instance)
	svcName := fmt.Sprintf("c2o-grafana-%s", instance)

	route := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "route.openshift.io/v1",
			"kind":       "Route",
			"metadata": map[string]any{
				"name":        routeName,
				"namespace":   namespace,
				"labels":      labels,
				"annotations": annotations,
			},
			"spec": map[string]any{
				"to": map[string]any{
					"kind": "Service",
					"name": svcName,
				},
				"port": map[string]any{
					"targetPort": "3000-tcp",
				},
				"tls": map[string]any{
					"termination":                   "edge",
					"insecureEdgeTerminationPolicy": "Redirect",
				},
			},
		},
	}

	_, err := dynClient.Resource(routeGVR).Namespace(namespace).Create(context.Background(), route, metav1.CreateOptions{})
	if err != nil && !isAlreadyExists(err) {
		return err
	}
	return nil
}

