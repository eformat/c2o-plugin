import * as React from 'react';
import {
  Title,
  Toolbar,
  ToolbarContent,
  ToolbarItem,
  FormSelect,
  FormSelectOption,
  Button,
  Spinner,
  Alert,
  EmptyState,
  EmptyStateBody,
  EmptyStateActions,
  EmptyStateFooter,
} from '@patternfly/react-core';
import {
  Table,
  Thead,
  Tr,
  Th,
  Tbody,
  Td,
} from '@patternfly/react-table';
import ConnectionDialog from './ConnectionDialog';
import { AgentInfo, ConnectionInfo, NamespaceInfo } from '../utils/types';
import * as api from '../utils/api';
import '../styles/c2o-plugin.css';

const ManagePage: React.FC = () => {
  const [namespace, setNamespace] = React.useState('');
  const [namespaces, setNamespaces] = React.useState<NamespaceInfo[]>([]);
  const [agents, setAgents] = React.useState<AgentInfo[]>([]);
  const [loading, setLoading] = React.useState(false);
  const [error, setError] = React.useState('');
  const [connection, setConnection] = React.useState<ConnectionInfo | null>(null);
  const [showConnection, setShowConnection] = React.useState(false);

  React.useEffect(() => {
    api.listNamespaces().then(setNamespaces).catch(() => {});
  }, []);

  React.useEffect(() => {
    if (namespace) {
      loadAgents();
    }
  }, [namespace]);

  const loadAgents = async () => {
    if (!namespace) return;
    setLoading(true);
    setError('');
    try {
      const a = await api.listAgents(namespace);
      setAgents(a);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (name: string) => {
    if (!confirm(`Delete agent ${name}? This will remove the deployment, services, and PVC.`)) return;
    try {
      await api.deleteAgent(name, namespace);
      await loadAgents();
    } catch (err: any) {
      setError(err.message);
    }
  };

  const handleShowConnection = async () => {
    try {
      const conn = await api.getConnection(namespace);
      setConnection(conn);
      setShowConnection(true);
    } catch (err: any) {
      setError(err.message);
    }
  };

  const statusDot = (status: string) => {
    const cls = status.toLowerCase().replace(/\s+/g, '-');
    return <span className={`c2o-status-dot c2o-status-dot--${cls}`} />;
  };

  return (
    <div className="c2o-page">
      <div className="c2o-page__header">
        <Title headingLevel="h1" size="2xl">
          Manage c2o Agents
        </Title>
        <p className="c2o-page__subtitle">
          View and manage your deployed coding agents
        </p>
      </div>

      <Toolbar>
        <ToolbarContent>
          <ToolbarItem>
            <FormSelect
              value={namespace}
              onChange={(_e, val) => setNamespace(val)}
              aria-label="Select namespace"
              style={{ minWidth: 200 }}
            >
              <FormSelectOption value="" label="-- Select namespace --" isDisabled />
              {namespaces.map((ns) => (
                <FormSelectOption key={ns.name} value={ns.name} label={ns.name} />
              ))}
            </FormSelect>
          </ToolbarItem>
          <ToolbarItem>
            <Button variant="secondary" onClick={loadAgents} isDisabled={!namespace}>
              Refresh
            </Button>
          </ToolbarItem>
          {agents.length > 0 && (
            <ToolbarItem>
              <Button variant="primary" onClick={handleShowConnection}>
                Connection Details
              </Button>
            </ToolbarItem>
          )}
          <ToolbarItem align={{ default: 'alignEnd' }}>
            <Button variant="primary" component="a" href="/c2o/deploy">
              Deploy New
            </Button>
          </ToolbarItem>
        </ToolbarContent>
      </Toolbar>

      {error && <Alert variant="danger" isInline title={error} style={{ marginTop: 16 }} />}

      {!namespace ? (
        <EmptyState>
          <EmptyStateBody>Select a namespace to view agents.</EmptyStateBody>
        </EmptyState>
      ) : loading ? (
        <div style={{ display: 'flex', justifyContent: 'center', padding: 48 }}>
          <Spinner size="xl" />
        </div>
      ) : agents.length === 0 ? (
        <EmptyState>
          <EmptyStateBody>No c2o agents found in namespace {namespace}.</EmptyStateBody>
          <EmptyStateFooter>
            <EmptyStateActions>
              <Button variant="primary" component="a" href="/c2o/deploy">
                Deploy Agents
              </Button>
            </EmptyStateActions>
          </EmptyStateFooter>
        </EmptyState>
      ) : (
        <Table aria-label="c2o agents" variant="compact" style={{ marginTop: 16 }}>
          <Thead>
            <Tr>
              <Th>Name</Th>
              <Th>Instance</Th>
              <Th>Type</Th>
              <Th>Status</Th>
              <Th>Image</Th>
              <Th>Age</Th>
              <Th>Actions</Th>
            </Tr>
          </Thead>
          <Tbody>
            {agents.map((agent) => (
              <Tr key={agent.name}>
                <Td dataLabel="Name">{agent.name}</Td>
                <Td dataLabel="Instance">{agent.instance}</Td>
                <Td dataLabel="Type">
                  <span style={{ textTransform: 'capitalize' }}>{agent.agentType}</span>
                </Td>
                <Td dataLabel="Status">
                  {statusDot(agent.status)}
                  {agent.status}
                </Td>
                <Td dataLabel="Image">
                  <code style={{ fontSize: 12 }}>{agent.image?.split('/').pop()}</code>
                </Td>
                <Td dataLabel="Age">{agent.age}</Td>
                <Td dataLabel="Actions">
                  <Button variant="danger" size="sm" onClick={() => handleDelete(agent.name)}>
                    Delete
                  </Button>
                </Td>
              </Tr>
            ))}
          </Tbody>
        </Table>
      )}

      <ConnectionDialog
        isOpen={showConnection}
        onClose={() => setShowConnection(false)}
        connection={connection}
        namespace={namespace}
      />
    </div>
  );
};

export default ManagePage;
