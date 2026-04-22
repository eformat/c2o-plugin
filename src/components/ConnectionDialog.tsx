import * as React from 'react';
import {
  Modal,
  ModalVariant,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Tabs,
  Tab,
  TabTitleText,
  Button,
  CodeBlock,
  CodeBlockCode,
  CodeBlockAction,
  ClipboardCopyButton,
  Alert,
  List,
  ListItem,
} from '@patternfly/react-core';
import { ConnectionInfo } from '../utils/types';

interface ConnectionDialogProps {
  isOpen: boolean;
  onClose: () => void;
  connection: ConnectionInfo | null;
  namespace: string;
}

const CopyableCodeBlock: React.FC<{ code: string; id: string }> = ({ code, id }) => {
  const [copied, setCopied] = React.useState(false);

  const handleCopy = () => {
    navigator.clipboard.writeText(code);
    setCopied(true);
  };

  return (
    <CodeBlock
      actions={
        <CodeBlockAction>
          <ClipboardCopyButton
            id={`copy-${id}`}
            textId={`code-${id}`}
            aria-label="Copy to clipboard"
            onClick={handleCopy}
            exitDelay={copied ? 1500 : 600}
            variant="plain"
            onTooltipHidden={() => setCopied(false)}
          >
            {copied ? 'Copied!' : 'Copy'}
          </ClipboardCopyButton>
        </CodeBlockAction>
      }
    >
      <CodeBlockCode id={`code-${id}`}>{code}</CodeBlockCode>
    </CodeBlock>
  );
};

const ConnectionDialog: React.FC<ConnectionDialogProps> = ({
  isOpen,
  onClose,
  connection,
  namespace,
}) => {
  const [activeTab, setActiveTab] = React.useState(0);

  if (!connection) return null;

  const grafanaEntries = Object.entries(connection.grafanaUrls || {});

  return (
    <Modal
      variant={ModalVariant.large}
      isOpen={isOpen}
      onClose={onClose}
    >
      <ModalHeader title={`Connect to c2o Agents — ${namespace}`} />
      <ModalBody>
        <div className="c2o-animate-in">
          <Alert
            variant="info"
            isInline
            isPlain
            title="Copy any of the connection methods below to connect your local Claude Code to the remote agents."
            style={{ marginBottom: 16 }}
          />

          <Tabs
            activeKey={activeTab}
            onSelect={(_, k) => setActiveTab(k as number)}
            isFilled
          >
            <Tab eventKey={0} title={<TabTitleText>MCP Config</TabTitleText>}>
              <div style={{ marginTop: 16 }}>
                <p style={{ marginBottom: 8, fontSize: 13, color: 'var(--pf-t--global--text--color--subtle)' }}>
                  Add this to your <code>.mcp.json</code> file:
                </p>
                <CopyableCodeBlock code={connection.mcpConfig} id="mcp-config" />
              </div>
            </Tab>

            <Tab eventKey={1} title={<TabTitleText>Skill Prompt</TabTitleText>}>
              <div style={{ marginTop: 16 }}>
                <p style={{ marginBottom: 8, fontSize: 13, color: 'var(--pf-t--global--text--color--subtle)' }}>
                  Save this as a <code>skill.md</code> file in your project:
                </p>
                <CopyableCodeBlock code={connection.skillPrompt} id="skill-prompt" />
              </div>
            </Tab>

            <Tab eventKey={2} title={<TabTitleText>Install CLI</TabTitleText>}>
              <div style={{ marginTop: 16 }}>
                <p style={{ marginBottom: 8, fontSize: 13, color: 'var(--pf-t--global--text--color--subtle)' }}>
                  Run these commands to set up the c2o MCP server locally:
                </p>
                <CopyableCodeBlock code={connection.installCmd} id="install-cmd" />
              </div>
            </Tab>

            {grafanaEntries.length > 0 && (
              <Tab eventKey={3} title={<TabTitleText>Grafana</TabTitleText>}>
                <div style={{ marginTop: 16 }}>
                  <p style={{ marginBottom: 12, fontSize: 13, color: 'var(--pf-t--global--text--color--subtle)' }}>
                    Agent dashboards with metrics, logs, and model routing stats:
                  </p>
                  <List>
                    {grafanaEntries.map(([instance, url]) => (
                      <ListItem key={instance}>
                        <a href={url} target="_blank" rel="noopener noreferrer">
                          <strong>{instance}</strong>
                        </a>
                        {' — '}
                        <span style={{ fontSize: 12, color: 'var(--pf-t--global--text--color--subtle)' }}>
                          {url}
                        </span>
                      </ListItem>
                    ))}
                  </List>
                </div>
              </Tab>
            )}
          </Tabs>
        </div>
      </ModalBody>
      <ModalFooter>
        <Button key="close" variant="primary" onClick={onClose}>
          Done
        </Button>
      </ModalFooter>
    </Modal>
  );
};

export default ConnectionDialog;
