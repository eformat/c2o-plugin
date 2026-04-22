import * as React from 'react';
import { Title } from '@patternfly/react-core';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import rehypeRaw from 'rehype-raw';
import '../styles/c2o-plugin.css';

const helpContent = `
## Getting Started

c2o (Claude-to-OpenShift) deploys AI coding agents in your OpenShift cluster. Each agent runs
Claude Code with full tool access — shell, file editing, and more — inside a secure pod.

### Quick Start

1. Go to **Deploy** and select your agent type
2. Choose a namespace you have access to
3. Set the number of agents (1-10)
4. Provide your API credentials
5. Click **Deploy** — your agents will be ready in ~60 seconds

### Connecting to Your Agents

After deployment, you'll get connection details in three formats:

#### MCP Config (recommended)
Add the JSON config to your \`.mcp.json\` file. Claude Code will automatically detect the
MCP server and make the agent tools available.

#### Skill Prompt
Save the prompt as a \`skill.md\` file in your project. Invoke it with \`/c2o-agents\` in
Claude Code.

#### CLI Install
Run the \`claude mcp add\` command to register the MCP server directly.

### How It Works

Each agent runs in its own pod with:
- **20Gi persistent workspace** — files survive restarts
- **Anthropic proxy** (port 8819) — translates API calls through the semantic router
- **OpenAI-compatible endpoint** (port 8899) — for tools that speak OpenAI format
- **Grafana + Prometheus** — monitoring dashboards

The MCP server on your local machine uses \`oc exec\` to send tasks to agent pods.
Tasks run as \`claude -p\` (pipe mode) with full tool permissions.

### Architecture

\`\`\`
Your Laptop                    OpenShift Cluster
┌─────────────────┐            ┌─────────────────────┐
│ Claude Code     │  oc exec   │ c2o-agent1 pod      │
│   └── MCP server│ ─────────→│   ├── claude -p      │
│       (local)   │            │   ├── anthropic proxy│
│                 │  oc exec   │   └── envoy + router │
│                 │ ─────────→│                       │
│                 │            │ c2o-agent2 pod      │
│                 │            │   └── ...            │
└─────────────────┘            └─────────────────────┘
\`\`\`

### Managing Agents

Use the **Manage** page to:
- View all agents across namespaces
- Check agent health and status
- Get connection details anytime
- Delete agents you no longer need

### Troubleshooting

| Issue | Solution |
|-------|----------|
| Agent stuck in Pending | Check namespace resource quotas |
| Tasks timing out | Increase proxy timeout (default 1200s) |
| Can't connect | Verify \`oc\` login and namespace access |
| Agent not responding | Check pod logs via \`oc logs\` |

### Resources

- [c2o Repository](https://github.com/rhai-code/vllm-sr-claude)
- [Claude Code Documentation](https://docs.anthropic.com/en/docs/claude-code)
- [MCP Protocol](https://modelcontextprotocol.io)
`;

const HelpPage: React.FC = () => {
  return (
    <div className="c2o-page">
      <div className="c2o-page__header">
        <Title headingLevel="h1" size="2xl">
          c2o Agents — Help
        </Title>
      </div>
      <div className="c2o-animate-in" style={{ maxWidth: 800 }}>
        <ReactMarkdown remarkPlugins={[remarkGfm]} rehypePlugins={[rehypeRaw]}>
          {helpContent}
        </ReactMarkdown>
      </div>
    </div>
  );
};

export default HelpPage;
