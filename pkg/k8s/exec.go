package k8s

import (
	"bytes"
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

func ExecInPod(token, namespace, pod, container string, command []string) (string, string, error) {
	cfg, err := baseConfig()
	if err != nil {
		return "", "", err
	}
	cfg.BearerToken = token
	cfg.BearerTokenFile = ""

	client, err := rest.RESTClientFor(setDefaults(cfg))
	if err != nil {
		return "", "", err
	}

	req := client.Post().
		Resource("pods").
		Name(pod).
		Namespace(namespace).
		SubResource("exec").
		Param("container", container).
		Param("stdout", "true").
		Param("stderr", "true")

	for _, c := range command {
		req = req.Param("command", c)
	}

	exec, err := remotecommand.NewSPDYExecutor(cfg, "POST", req.URL())
	if err != nil {
		return "", "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})

	return stdout.String(), stderr.String(), err
}

func setDefaults(cfg *rest.Config) *rest.Config {
	cp := *cfg
	cp.APIPath = "/api"
	cp.GroupVersion = &corev1.SchemeGroupVersion
	cp.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
	return &cp
}
