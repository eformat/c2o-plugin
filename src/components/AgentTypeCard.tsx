import * as React from 'react';
import { AgentTypeOption } from '../utils/types';
import claudeIcon from '../assets/icons/claude.svg';
import codexIcon from '../assets/icons/codex.svg';
import opencodeIcon from '../assets/icons/opencode.svg';

const iconMap: Record<string, string> = {
  claude: claudeIcon,
  codex: codexIcon,
  opencode: opencodeIcon,
};

interface AgentTypeCardProps {
  agent: AgentTypeOption;
  selected: boolean;
  onSelect: (id: string) => void;
}

const AgentTypeCard: React.FC<AgentTypeCardProps> = ({ agent, selected, onSelect }) => {
  const classes = [
    'c2o-agent-card',
    selected ? 'c2o-agent-card--selected' : '',
    !agent.enabled ? 'c2o-agent-card--disabled' : '',
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <div
      className={classes}
      onClick={() => agent.enabled && onSelect(agent.id)}
      role="button"
      tabIndex={agent.enabled ? 0 : -1}
      aria-pressed={selected}
      aria-disabled={!agent.enabled}
      onKeyDown={(e) => {
        if (agent.enabled && (e.key === 'Enter' || e.key === ' ')) {
          e.preventDefault();
          onSelect(agent.id);
        }
      }}
    >
      {selected && (
        <div className="c2o-agent-card__check">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="currentColor">
            <path d="M13.78 4.22a.75.75 0 010 1.06l-7.25 7.25a.75.75 0 01-1.06 0L2.22 9.28a.75.75 0 011.06-1.06L6 10.94l6.72-6.72a.75.75 0 011.06 0z" />
          </svg>
        </div>
      )}

      <div className="c2o-agent-card__icon">
        <img src={iconMap[agent.id]} alt={agent.name} width="48" height="48" />
      </div>

      <h3 className="c2o-agent-card__name">{agent.name}</h3>
      <p className="c2o-agent-card__description">{agent.description}</p>

      {agent.comingSoon && <span className="c2o-badge-coming-soon">Coming Soon</span>}
    </div>
  );
};

export default AgentTypeCard;
