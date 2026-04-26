import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {
  Button,
  Tooltip,
} from '@patternfly/react-core';
import { CloseIcon } from '@patternfly/react-icons/dist/esm/icons/close-icon';
import { CompressIcon } from '@patternfly/react-icons/dist/esm/icons/compress-icon';
import { ExpandIcon } from '@patternfly/react-icons/dist/esm/icons/expand-icon';
import TerminalSession from './TerminalSession';

interface TerminalDrawerProps {
  agents: string[];
  activeAgent: string;
  namespace: string;
  onSelectTab: (name: string) => void;
  onCloseTab: (name: string) => void;
  onCloseAll: () => void;
}

const DEFAULT_HEIGHT = 400;
const MIN_HEIGHT = 48;

const TerminalDrawer: React.FC<TerminalDrawerProps> = ({
  agents,
  activeAgent,
  namespace,
  onSelectTab,
  onCloseTab,
  onCloseAll,
}) => {
  const [expanded, setExpanded] = React.useState(true);
  const [height, setHeight] = React.useState(DEFAULT_HEIGHT);
  const drawerRef = React.useRef<HTMLDivElement>(null);
  const dragStartRef = React.useRef<{ y: number; h: number } | null>(null);

  const handleDragStart = React.useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    const currentH = drawerRef.current?.offsetHeight || height;
    dragStartRef.current = { y: e.clientY, h: currentH };

    const handleDragMove = (ev: MouseEvent) => {
      if (!dragStartRef.current) return;
      const delta = dragStartRef.current.y - ev.clientY;
      const newHeight = Math.max(MIN_HEIGHT, dragStartRef.current.h + delta);
      const maxH = window.innerHeight - 60;
      setHeight(Math.min(newHeight, maxH));
      setExpanded(newHeight > MIN_HEIGHT);
    };

    const handleDragEnd = () => {
      dragStartRef.current = null;
      document.removeEventListener('mousemove', handleDragMove);
      document.removeEventListener('mouseup', handleDragEnd);
    };

    document.addEventListener('mousemove', handleDragMove);
    document.addEventListener('mouseup', handleDragEnd);
  }, [height]);

  if (agents.length === 0) return null;

  return ReactDOM.createPortal(
    <div
      ref={drawerRef}
      className="c2o-drawer"
      style={{ height: expanded ? height : MIN_HEIGHT }}
    >
      <div className="c2o-drawer__drag-handle" onMouseDown={handleDragStart} />

      <div className="c2o-drawer__header">
        <div className="c2o-drawer__tabs">
          {agents.map((name) => (
            <div
              key={name}
              className={`c2o-drawer__tab ${name === activeAgent ? 'c2o-drawer__tab--active' : ''}`}
              onClick={() => onSelectTab(name)}
            >
              <span className="c2o-drawer__tab-label">{name}</span>
              <button
                className="c2o-drawer__tab-close"
                onClick={(e) => { e.stopPropagation(); onCloseTab(name); }}
                aria-label={`Close ${name}`}
              >
                ×
              </button>
            </div>
          ))}
        </div>
        <div className="c2o-drawer__actions">
          <Tooltip content={expanded ? 'Minimize terminal' : 'Restore terminal'}>
            <Button
              variant="plain"
              aria-label={expanded ? 'Minimize' : 'Restore'}
              onClick={() => setExpanded(!expanded)}
            >
              {expanded ? <CompressIcon /> : <ExpandIcon />}
            </Button>
          </Tooltip>
          <Tooltip content="Close all terminals">
            <Button variant="plain" aria-label="Close all terminals" onClick={onCloseAll}>
              <CloseIcon />
            </Button>
          </Tooltip>
        </div>
      </div>

      {expanded && (
        <div className="c2o-drawer__body">
          {agents.map((name) => (
            <TerminalSession
              key={name}
              agentName={name}
              namespace={namespace}
              isActive={name === activeAgent}
            />
          ))}
        </div>
      )}
    </div>,
    document.body,
  );
};

export default TerminalDrawer;
