package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/rhai-code/c2o-plugin/pkg/k8s"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ConnectionInfo struct {
	MCPConfig   string            `json:"mcpConfig"`
	SkillPrompt string            `json:"skillPrompt"`
	InstallCmd  string            `json:"installCmd"`
	GrafanaURLs map[string]string `json:"grafanaUrls"`
}

// GetConnection returns MCP config and skill prompt for connecting to c2o agents.
func GetConnection(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("X-User-Token")
	namespace := r.URL.Query().Get("namespace")
	if namespace == "" {
		httpError(w, http.StatusBadRequest, "namespace parameter required")
		return
	}

	mcpConfig := fmt.Sprintf(`{
  "mcpServers": {
    "c2o-agents": {
      "command": "python3",
      "args": ["c2o-mcp-server.py"],
      "env": {
        "C2O_NAMESPACE": "%s"
      }
    }
  }
}`, namespace)

	skillPrompt := fmt.Sprintf(`---
name: c2o-agents
description: Orchestrate remote c2o coding agents in OpenShift namespace %s
---

You have access to c2o coding agents deployed in OpenShift namespace "%s".

Use the MCP tools to:
- list_agents: List all available c2o agent instances
- send_task: Send a coding task to a specific agent
- get_task_result: Retrieve completed task results
- exec_on_agent: Run commands on an agent pod
- get_agent_logs: View agent pod logs

Example workflow:
1. List available agents
2. Send a task to an agent: "Analyze the codebase and create a summary"
3. Check task status and retrieve results when complete
`, namespace, namespace)

	installCmd := fmt.Sprintf(`# 1. Download the MCP server script
curl -sLO https://raw.githubusercontent.com/eformat/vllm-sr-claude/main/hack/c2o-mcp-server.py

# 2. Install the MCP Python dependency
pip install mcp

# 3. Register with Claude Code
claude mcp add c2o-agents -e C2O_NAMESPACE=%s -- python3 c2o-mcp-server.py`, namespace)

	// Discover Grafana routes
	grafanaURLs := map[string]string{}
	dynClient, err := k8s.DynamicClientFromToken(token)
	if err == nil {
		gvr := schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
		routes, err := dynClient.Resource(gvr).Namespace(namespace).List(context.Background(), metav1.ListOptions{
			LabelSelector: "app=c2o",
		})
		if err == nil {
			for _, route := range routes.Items {
				name := route.GetName()
				spec, ok := route.Object["spec"].(map[string]interface{})
				if !ok {
					continue
				}
				host, _ := spec["host"].(string)
				if host != "" && len(name) > len("c2o-grafana-") && name[:len("c2o-grafana-")] == "c2o-grafana-" {
					instance := name[len("c2o-grafana-"):]
					grafanaURLs[instance] = fmt.Sprintf("https://%s", host)
				}
			}
		} else {
			slog.Warn("failed to list routes", "error", err)
		}
	}

	jsonResponse(w, ConnectionInfo{
		MCPConfig:   mcpConfig,
		SkillPrompt: skillPrompt,
		InstallCmd:  installCmd,
		GrafanaURLs: grafanaURLs,
	})
}
