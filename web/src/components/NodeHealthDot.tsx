import React from 'react';

export type NodeHealthStatus =
  | 'alive'
  | 'suspect'
  | 'offline'
  | 'healthy'
  | 'warning'
  | 'degraded'
  | 'unknown';

export interface NodeHealthDotProps {
  status: NodeHealthStatus;
  className?: string;
  showPing?: boolean;
}

export const NodeHealthDot: React.FC<NodeHealthDotProps> = ({
  status,
  className = '',
  showPing = true,
}) => {
  const isHealthy = status === 'alive' || status === 'healthy';
  const isWarning = status === 'suspect' || status === 'warning';
  const isDegraded = status === 'offline' || status === 'degraded';

  const title =
    status === 'alive'
      ? 'Alive'
      : status === 'healthy'
      ? 'Healthy'
      : status === 'suspect'
      ? 'Suspect'
      : status === 'warning'
      ? 'Warning'
      : status === 'offline'
      ? 'Offline'
      : status === 'degraded'
      ? 'Degraded'
      : 'Unknown';

  if (isHealthy) {
    return (
      <span className={`relative flex h-2 w-2 ${className}`.trim()} title={title}>
        {showPing && (
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
        )}
        <span className="relative inline-flex rounded-full h-2 w-2 bg-emerald-500" />
      </span>
    );
  }

  if (isWarning) {
    return (
      <span
        className={`relative inline-flex rounded-full h-2 w-2 bg-amber-400 ${className}`.trim()}
        title={title}
      />
    );
  }

  if (isDegraded) {
    return (
      <span
        className={`relative inline-flex rounded-full h-2 w-2 bg-rose-500 ${className}`.trim()}
        title={title}
      />
    );
  }

  return (
    <span
      className={`relative inline-flex rounded-full h-2 w-2 bg-slate-500 ${className}`.trim()}
      title={title}
    />
  );
};
