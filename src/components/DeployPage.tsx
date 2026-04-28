import * as React from 'react';
import {
  Title,
  Wizard,
  WizardStep,
  FormGroup,
  TextInput,
  NumberInput,
  Alert,
  Spinner,
  Button,
  FormSelect,
  FormSelectOption,
  FileUpload,
  Radio,
  ActionGroup,
} from '@patternfly/react-core';
import AgentTypeCard from './AgentTypeCard';
import ConnectionDialog from './ConnectionDialog';
import { AGENT_TYPES, AgentType, ConnectionInfo, CredentialInfo, NamespaceInfo } from '../utils/types';
import * as api from '../utils/api';
import '../styles/c2o-plugin.css';

const CREATE_NS_VALUE = '__create_new__';

const DeployPage: React.FC = () => {
  // Wizard state
  const [agentType, setAgentType] = React.useState<AgentType>('claude');
  const [namespace, setNamespace] = React.useState('');
  const [namespaces, setNamespaces] = React.useState<NamespaceInfo[]>([]);
  const [count, setCount] = React.useState(1);
  const [prefix, setPrefix] = React.useState('agent');

  // Create namespace state
  const [creatingNs, setCreatingNs] = React.useState(false);
  const [newNsName, setNewNsName] = React.useState('');
  const [nsError, setNsError] = React.useState('');

  // Credentials state
  const [credMode, setCredMode] = React.useState<'new' | 'existing'>('new');
  const [apiKey, setApiKey] = React.useState('');
  const [gcpProjectId, setGcpProjectId] = React.useState('');
  const [token, setToken] = React.useState('');
  const [kimiHost, setKimiHost] = React.useState('');
  const [gcpFile, setGcpFile] = React.useState('');
  const [gcpFilename, setGcpFilename] = React.useState('');
  const [existingCreds, setExistingCreds] = React.useState<CredentialInfo[]>([]);
  const [selectedCred, setSelectedCred] = React.useState('');

  // UI state
  const [loading, setLoading] = React.useState(false);
  const [loadingNs, setLoadingNs] = React.useState(true);
  const [error, setError] = React.useState('');
  const [deployed, setDeployed] = React.useState(false);
  const [connection, setConnection] = React.useState<ConnectionInfo | null>(null);
  const [showConnection, setShowConnection] = React.useState(false);

  React.useEffect(() => {
    const loadProjects = (retries = 2) => {
      api.listNamespaces().then((ns) => {
        setNamespaces(ns);
        setLoadingNs(false);
        setError('');
      }).catch((err) => {
        if (retries > 0) {
          setTimeout(() => loadProjects(retries - 1), 1500);
        } else {
          setError(`Failed to load projects: ${err.message}`);
          setLoadingNs(false);
        }
      });
    };
    loadProjects();
  }, []);

  React.useEffect(() => {
    if (namespace && credMode === 'existing') {
      api.listCredentials(namespace).then(setExistingCreds).catch(() => {});
    }
  }, [namespace, credMode]);

  const credentialName = React.useMemo(() => {
    if (credMode === 'existing') return selectedCred;
    return 'c2o-env';
  }, [credMode, selectedCred, prefix]);

  const handleNsChange = (_e: any, val: string) => {
    if (val === CREATE_NS_VALUE) {
      setCreatingNs(true);
      setNamespace('');
    } else {
      setCreatingNs(false);
      setNamespace(val);
    }
  };

  const handleCreateNs = async () => {
    if (!newNsName.trim()) return;
    setNsError('');
    try {
      const name = newNsName.trim();
      await api.createNamespace(name);
      setNamespaces((prev) => [...prev, { name, status: 'Active' }]);
      setNamespace(name);
      setCreatingNs(false);
      setNewNsName('');
      setError('');
      api.listNamespaces().then(setNamespaces).catch(() => {});
    } catch (err: any) {
      setNsError(err.message || 'Failed to create project');
    }
  };

  const handleDeploy = async () => {
    setLoading(true);
    setError('');

    try {
      // Create credentials if new
      if (credMode === 'new') {
        const data: Record<string, string> = {};
        if (apiKey) data['ANTHROPIC_API_KEY'] = apiKey;
        if (gcpProjectId) data['GCP_PROJECT_ID'] = gcpProjectId;
        if (token) data['TOKEN'] = token;
        if (kimiHost) data['KIMI_HOST'] = kimiHost;
        if (gcpFile) data['GOOGLE_APPLICATION_CREDENTIALS_JSON'] = gcpFile;

        await api.createCredentials({
          namespace,
          name: credentialName,
          type: 'apikey',
          data,
        });
      }

      // Deploy agents
      await api.deploy({
        agentType,
        namespace,
        count,
        prefix,
        credentialName,
        image: '',
      });

      // Get connection info
      const conn = await api.getConnection(namespace);
      setConnection(conn);
      setDeployed(true);
      setShowConnection(true);
    } catch (err: any) {
      setError(err.message || 'Deployment failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="c2o-page">
      <div className="c2o-page__header">
        <Title headingLevel="h1" size="2xl">
          Deploy c2o Agents
        </Title>
        <p className="c2o-page__subtitle">
          Deploy coding agents to your OpenShift namespace
        </p>
      </div>

      {error && (
        <Alert variant="danger" isInline title={error} style={{ marginBottom: 16 }} />
      )}

      {deployed ? (
        <div className="c2o-animate-in">
          <Alert variant="success" isInline title="Agents deployed successfully!" style={{ marginBottom: 16 }}>
            <p>Your agents are being created in namespace <strong>{namespace}</strong>.</p>
          </Alert>
          <ActionGroup>
            <Button variant="primary" onClick={() => setShowConnection(true)}>
              View Connection Details
            </Button>
            <Button variant="secondary" component="a" href="/c2o/manage">
              Go to Manage
            </Button>
          </ActionGroup>
          <ConnectionDialog
            isOpen={showConnection}
            onClose={() => setShowConnection(false)}
            connection={connection}
            namespace={namespace}
          />
        </div>
      ) : (
        <Wizard
          title="Deploy Agents"
          onClose={() => window.history.back()}
          onSave={handleDeploy}
        >
          <WizardStep name="Agent Type" id="agent-type">
            <div className="c2o-wizard-step c2o-animate-in">
              <Title headingLevel="h3" size="lg" style={{ marginBottom: 16 }}>
                Select Agent Type
              </Title>
              <div className="c2o-card-grid">
                {AGENT_TYPES.map((agent) => (
                  <AgentTypeCard
                    key={agent.id}
                    agent={agent}
                    selected={agentType === agent.id}
                    onSelect={(id) => setAgentType(id as AgentType)}
                  />
                ))}
              </div>
            </div>
          </WizardStep>

          <WizardStep name="Namespace" id="namespace">
            <div className="c2o-wizard-step c2o-animate-in">
              <Title headingLevel="h3" size="lg" style={{ marginBottom: 16 }}>
                Select Namespace
              </Title>
              {loadingNs ? (
                <Spinner size="lg" />
              ) : (
                <>
                  <FormGroup label="Namespace" isRequired fieldId="namespace-select">
                    <FormSelect
                      id="namespace-select"
                      value={creatingNs ? CREATE_NS_VALUE : namespace}
                      onChange={handleNsChange}
                      aria-label="Select namespace"
                    >
                      <FormSelectOption value="" label="-- Select a namespace --" isDisabled />
                      <FormSelectOption value={CREATE_NS_VALUE} label="+ Create new namespace" />
                      {namespaces.map((ns) => (
                        <FormSelectOption key={ns.name} value={ns.name} label={ns.name} />
                      ))}
                    </FormSelect>
                  </FormGroup>

                  {creatingNs && (
                    <div style={{ marginTop: 16 }} className="c2o-animate-in">
                      {nsError && (
                        <Alert variant="danger" isInline isPlain title={nsError} style={{ marginBottom: 8 }} />
                      )}
                      <FormGroup label="New namespace name" isRequired fieldId="new-ns-name">
                        <div style={{ display: 'flex', gap: 8 }}>
                          <TextInput
                            id="new-ns-name"
                            value={newNsName}
                            onChange={(_e, val) => setNewNsName(val)}
                            placeholder="e.g. user-jdoe"
                            style={{ flex: 1 }}
                          />
                          <Button variant="primary" onClick={handleCreateNs} isDisabled={!newNsName.trim()}>
                            Create
                          </Button>
                        </div>
                      </FormGroup>
                    </div>
                  )}
                </>
              )}
            </div>
          </WizardStep>

          <WizardStep name="Configuration" id="config">
            <div className="c2o-wizard-step c2o-animate-in">
              <Title headingLevel="h3" size="lg" style={{ marginBottom: 16 }}>
                Agent Configuration
              </Title>
              <FormGroup label="Number of agents" fieldId="agent-count">
                <NumberInput
                  id="agent-count"
                  value={count}
                  min={1}
                  max={10}
                  onMinus={() => setCount(Math.max(1, count - 1))}
                  onPlus={() => setCount(Math.min(10, count + 1))}
                  onChange={(e) => {
                    const val = parseInt((e.target as HTMLInputElement).value, 10);
                    if (!isNaN(val) && val >= 1 && val <= 10) setCount(val);
                  }}
                />
              </FormGroup>
              <FormGroup label="Instance name prefix" fieldId="prefix" style={{ marginTop: 16 }}>
                <TextInput
                  id="prefix"
                  value={prefix}
                  onChange={(_e, val) => setPrefix(val)}
                  placeholder="agent"
                />
                <p style={{ fontSize: 12, color: 'var(--pf-t--global--text--color--subtle)', marginTop: 4 }}>
                  Agents will be named: {Array.from({ length: Math.min(count, 3) }, (_, i) =>
                    `c2o-${prefix}${i + 1}`
                  ).join(', ')}{count > 3 ? ', ...' : ''}
                </p>
              </FormGroup>
            </div>
          </WizardStep>

          <WizardStep name="Credentials" id="credentials">
            <div className="c2o-wizard-step c2o-animate-in">
              <Title headingLevel="h3" size="lg" style={{ marginBottom: 16 }}>
                Credentials
              </Title>

              <FormGroup fieldId="cred-mode" style={{ marginBottom: 16 }}>
                <Radio
                  id="cred-new"
                  name="cred-mode"
                  label="Create new credentials"
                  isChecked={credMode === 'new'}
                  onChange={() => setCredMode('new')}
                />
                <Radio
                  id="cred-existing"
                  name="cred-mode"
                  label="Use existing secret"
                  isChecked={credMode === 'existing'}
                  onChange={() => setCredMode('existing')}
                  style={{ marginTop: 8 }}
                />
              </FormGroup>

              {credMode === 'new' ? (
                <>
                  <Title headingLevel="h4" size="md" style={{ marginBottom: 8 }}>
                    Claude / Anthropic
                  </Title>
                  <FormGroup label="Anthropic API Key" fieldId="api-key" style={{ marginBottom: 12 }}>
                    <TextInput
                      id="api-key"
                      type="password"
                      value={apiKey}
                      onChange={(_e, val) => setApiKey(val)}
                      placeholder="sk-ant-..."
                    />
                  </FormGroup>
                  <FormGroup label="GCP Project ID" fieldId="gcp-project" style={{ marginBottom: 12 }}>
                    <TextInput
                      id="gcp-project"
                      value={gcpProjectId}
                      onChange={(_e, val) => setGcpProjectId(val)}
                      placeholder="my-gcp-project"
                    />
                    <p style={{ fontSize: 12, color: 'var(--pf-t--global--text--color--subtle)', marginTop: 4 }}>
                      Required for Claude models via Vertex AI
                    </p>
                  </FormGroup>

                  <Title headingLevel="h4" size="md" style={{ marginTop: 20, marginBottom: 8 }}>
                    Kimi / MaaS (optional)
                  </Title>
                  <FormGroup label="Token" fieldId="kimi-token" style={{ marginBottom: 12 }}>
                    <TextInput
                      id="kimi-token"
                      type="password"
                      value={token}
                      onChange={(_e, val) => setToken(val)}
                      placeholder="Kimi MaaS API token"
                    />
                  </FormGroup>
                  <FormGroup label="Kimi Host" fieldId="kimi-host" style={{ marginBottom: 12 }}>
                    <TextInput
                      id="kimi-host"
                      value={kimiHost}
                      onChange={(_e, val) => setKimiHost(val)}
                      placeholder="maas.example.com"
                    />
                  </FormGroup>

                  <Title headingLevel="h4" size="md" style={{ marginTop: 20, marginBottom: 8 }}>
                    GCP Service Account (optional)
                  </Title>
                  <FormGroup label="Service Account JSON" fieldId="gcp-file">
                    <FileUpload
                      id="gcp-file"
                      type="text"
                      value={gcpFile}
                      filename={gcpFilename}
                      onTextChange={(_e, val) => setGcpFile(val)}
                      onFileInputChange={(_e, file) => setGcpFilename(file.name)}
                      onDataChange={(_e, val) => setGcpFile(val)}
                      browseButtonText="Upload JSON"
                    />
                  </FormGroup>
                </>
              ) : (
                <FormGroup label="Existing secret" fieldId="existing-cred">
                  {existingCreds.length > 0 ? (
                    <FormSelect
                      id="existing-cred"
                      value={selectedCred}
                      onChange={(_e, val) => setSelectedCred(val)}
                    >
                      <FormSelectOption value="" label="-- Select a secret --" isDisabled />
                      {existingCreds.map((c) => (
                        <FormSelectOption
                          key={c.name}
                          value={c.name}
                          label={`${c.name} (${c.type})`}
                        />
                      ))}
                    </FormSelect>
                  ) : (
                    <Alert variant="info" isInline isPlain title="No c2o credential secrets found in this namespace." />
                  )}
                </FormGroup>
              )}
            </div>
          </WizardStep>

          <WizardStep name="Review & Deploy" id="review">
            <div className="c2o-wizard-step c2o-animate-in">
              <Title headingLevel="h3" size="lg" style={{ marginBottom: 16 }}>
                Review
              </Title>

              <dl className="c2o-deploy-summary">
                <dt>Agent Type</dt>
                <dd>{AGENT_TYPES.find((a) => a.id === agentType)?.name}</dd>

                <dt>Namespace</dt>
                <dd>{namespace || '—'}</dd>

                <dt>Agents</dt>
                <dd>{count} instance{count > 1 ? 's' : ''}</dd>

                <dt>Prefix</dt>
                <dd>{prefix}</dd>

                <dt>Credentials</dt>
                <dd>{credMode === 'existing' ? selectedCred : `${credentialName} (new)`}</dd>
              </dl>

              {loading && (
                <div style={{ marginTop: 16, display: 'flex', alignItems: 'center', gap: 8 }}>
                  <Spinner size="md" />
                  <span>Deploying agents...</span>
                </div>
              )}
            </div>
          </WizardStep>
        </Wizard>
      )}
    </div>
  );
};

export default DeployPage;
