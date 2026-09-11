import React, { useState, useMemo, useDeferredValue } from 'react';
import { ClusterContainer } from '../types';
import { useClusterContainersQuery } from '../hooks/useClusterData';
import { getHostForContainer } from '../lib/utils';
import { ContainerDetailSheet } from './ContainerDetailSheet';
import { GenerateComposeDialog } from './GenerateComposeDialog';
import { ContainerLogsModal } from './ContainerLogsModal';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Input } from './ui/input';
import {
  Box,
  Server,
  Layers,
  Sparkles,
  Search,
  AlertCircle,
  AlertTriangle,
  Loader2,
  ExternalLink,
  X,
  Terminal,
} from 'lucide-react';

export interface ClusterContainersViewProps {
  selectedHostId?: string | null;
  searchQuery?: string;
}

export const ClusterContainersView: React.FC<ClusterContainersViewProps> = ({
  selectedHostId,
  searchQuery: initialSearch = '',
}) => {
  const [stateFilter, setStateFilter] = useState<'all' | 'running' | 'exited' | 'paused'>('all');
  const [typeFilter, setTypeFilter] = useState<'all' | 'standalone' | 'stack'>('all');
  const [localSearch, setLocalSearch] = useState('');
  const [selectedContainer, setSelectedContainer] = useState<ClusterContainer | null>(null);
  const [generatingForContainer, setGeneratingForContainer] = useState<ClusterContainer | null>(null);
  const [logsForContainer, setLogsForContainer] = useState<ClusterContainer | null>(null);
  const [dismissedUnreachable, setDismissedUnreachable] = useState(false);

  const { data, isLoading, error } = useClusterContainersQuery(selectedHostId);
  const containers = data?.items ?? [];
  const unreachableNodes = data?.unreachableNodes ?? [];

  const rawSearch = initialSearch || localSearch;
  const deferredSearch = useDeferredValue(rawSearch);
  const activeSearch = deferredSearch.trim().toLowerCase();

  const filteredContainers = useMemo(() => {
    return containers.filter((container) => {
      // State filter
      if (stateFilter !== 'all') {
        const state = container.state.toLowerCase();
        if (stateFilter === 'running' && state !== 'running') return false;
        if (stateFilter === 'exited' && state !== 'exited' && state !== 'stopped') return false;
        if (stateFilter === 'paused' && state !== 'paused') return false;
      }

      // Type filter
      if (typeFilter === 'standalone' && container.stack) return false;
      if (typeFilter === 'stack' && !container.stack) return false;

      // Search query
      if (activeSearch) {
        const cleanName = container.name.replace(/^\//, '').toLowerCase();
        const matchName = cleanName.includes(activeSearch);
        const matchImage = container.image.toLowerCase().includes(activeSearch);
        const matchHost = container.hostName.toLowerCase().includes(activeSearch);
        const matchStack = (container.stack || '').toLowerCase().includes(activeSearch);
        const matchService = (container.service || '').toLowerCase().includes(activeSearch);
        if (!matchName && !matchImage && !matchHost && !matchStack && !matchService) {
          return false;
        }
      }

      return true;
    });
  }, [containers, stateFilter, typeFilter, activeSearch]);

  const runningCount = useMemo(
    () => containers.filter((c) => c.state === 'running').length,
    [containers]
  );

  return (
    <div className="space-y-4">
      {/* Reachability Warning Banner */}
      {!dismissedUnreachable && unreachableNodes.length > 0 && (
        <div className="p-3.5 bg-amber-950/40 border border-amber-800/60 rounded-xl text-amber-200 text-xs flex items-start justify-between gap-3 shadow-sm">
          <div className="flex items-start gap-2.5 min-w-0">
            <AlertTriangle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
            <div className="space-y-1 min-w-0 leading-relaxed">
              <div className="font-semibold text-amber-300">
                Cluster Node Reachability Warning
              </div>
              <div className="text-slate-300">
                {unreachableNodes.length === 1 ? 'A cluster node' : `${unreachableNodes.length} cluster nodes`} failed to respond or timed out. Containers from {unreachableNodes.length === 1 ? 'this node' : 'these nodes'} may be omitted:
              </div>
              <div className="flex flex-wrap gap-1.5 pt-1">
                {unreachableNodes.map((node) => (
                  <span
                    key={node.id}
                    className="inline-flex items-center gap-1 px-2 py-0.5 rounded bg-amber-900/40 text-amber-200 border border-amber-700/50 font-mono text-[11px] tabular-nums"
                    title={node.error ? `Error: ${node.error}` : undefined}
                  >
                    <Server className="w-3 h-3 text-amber-400" />
                    {node.name}
                    {node.error && <span className="opacity-75 text-[10px]">({node.error})</span>}
                  </span>
                ))}
              </div>
            </div>
          </div>
          <button
            type="button"
            onClick={() => setDismissedUnreachable(true)}
            aria-label="Dismiss reachability warning"
            className="text-amber-400 hover:text-amber-200 p-1 rounded-md hover:bg-amber-900/40 transition-colors flex-shrink-0 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-amber-400"
          >
            <X className="w-4 h-4" />
          </button>
        </div>
      )}

      {/* Top Controls Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Box className="w-5 h-5 text-rose-500" />
            Containers
          </h2>
          <Badge variant="outline" className="text-xs font-mono tabular-nums bg-slate-900 border-slate-700 text-slate-300">
            {runningCount} running • {filteredContainers.length} total
          </Badge>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-2">
          {/* State Filter Pills */}
          <div className="flex items-center rounded-lg bg-slate-900 border border-slate-800 p-0.5 text-xs">
            <button
              onClick={() => setStateFilter('all')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                stateFilter === 'all'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              All States
            </button>
            <button
              onClick={() => setStateFilter('running')}
              className={`px-2.5 py-1 rounded-md transition-all flex items-center gap-1.5 ${
                stateFilter === 'running'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
              Running
            </button>
            <button
              onClick={() => setStateFilter('exited')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                stateFilter === 'exited'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Exited
            </button>
            <button
              onClick={() => setStateFilter('paused')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                stateFilter === 'paused'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Paused
            </button>
          </div>

          {/* Type Filter Pills */}
          <div className="flex items-center rounded-lg bg-slate-900 border border-slate-800 p-0.5 text-xs">
            <button
              onClick={() => setTypeFilter('all')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                typeFilter === 'all'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              All
            </button>
            <button
              onClick={() => setTypeFilter('stack')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                typeFilter === 'stack'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              In Stack
            </button>
            <button
              onClick={() => setTypeFilter('standalone')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                typeFilter === 'standalone'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Standalone
            </button>
          </div>

          {/* Local search if navbar search is empty */}
          {!initialSearch && (
            <div className="relative">
              <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
              <Input
                value={localSearch}
                onChange={(e) => setLocalSearch(e.target.value)}
                placeholder="Filter containers…"
                className="h-8 pl-8 pr-2.5 text-xs bg-slate-900 border-slate-800 w-36 sm:w-44 focus-visible:ring-1 focus-visible:ring-rose-500 text-slate-200"
              />
            </div>
          )}
        </div>
      </div>

      {error && (
        <div className="p-4 bg-rose-950/40 border border-rose-800/60 rounded-xl text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
          <span>Failed to load containers: {(error as Error).message}</span>
        </div>
      )}

      {isLoading ? (
        <div className="p-16 text-center text-xs text-slate-400 flex flex-col items-center justify-center gap-2">
          <Loader2 className="w-6 h-6 animate-spin text-rose-500" />
          <span>Loading containers…</span>
        </div>
      ) : filteredContainers.length === 0 ? (
        <div className="p-12 border border-slate-800 rounded-xl text-center text-xs text-slate-500 bg-slate-950/40">
          No containers found matching your filters.
        </div>
      ) : (
        <div className="border border-slate-800 rounded-xl overflow-hidden bg-slate-950/40">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-900/80 text-slate-400 font-mono text-[11px] uppercase tracking-wider border-b border-slate-800">
              <tr>
                <th className="p-3.5">Container</th>
                <th className="p-3.5">Host</th>
                <th className="p-3.5">State</th>
                <th className="p-3.5">Stack / Service</th>
                <th className="p-3.5">Image</th>
                <th className="p-3.5">Ports</th>
                <th className="p-3.5 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-mono">
              {filteredContainers.map((container) => {
                const cleanName = container.name.replace(/^\//, '');
                const isRunning = container.state === 'running';
                const isPaused = container.state === 'paused';
                const isCompleted =
                  container.state === 'exited' &&
                  (container.exit_code === 0 || container.status?.toLowerCase().startsWith('exited (0)'));
                const isStandalone = !container.stack;

                return (
                  <tr
                    key={`${container.hostId}-${container.id}`}
                    role="button"
                    tabIndex={0}
                    onClick={() => setSelectedContainer(container)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        setSelectedContainer(container);
                      }
                    }}
                    className="hover:bg-slate-800/40 cursor-pointer transition-colors group focus-visible:outline-none focus-visible:bg-slate-800/60"
                  >
                    <td className="p-3.5 font-sans">
                      <div className="flex items-center gap-2">
                        <span
                          className={`w-2 h-2 rounded-full flex-shrink-0 ${
                            isRunning
                              ? 'bg-emerald-400 ring-2 ring-emerald-400/20'
                              : isCompleted
                              ? 'bg-blue-400 ring-2 ring-blue-400/20'
                              : isPaused
                              ? 'bg-amber-400'
                              : 'bg-slate-600'
                          }`}
                          title={isRunning ? 'Running' : isCompleted ? 'Completed (Exit 0)' : isPaused ? 'Paused' : 'Stopped'}
                        />
                        <div>
                          <div className="font-semibold text-slate-100 group-hover:text-white">
                            {cleanName}
                          </div>
                          <div className="text-[10px] text-slate-500 font-mono tabular-nums">
                            {container.id.substring(0, 12)}
                          </div>
                        </div>
                      </div>
                    </td>

                    <td className="p-3.5">
                      <Badge variant="outline" className="text-[10px] bg-slate-900 text-slate-300 border-slate-800 font-mono flex items-center gap-1 w-fit">
                        <Server className="w-3 h-3 text-slate-500" />
                        {container.hostName}
                      </Badge>
                    </td>

                    <td className="p-3.5">
                      <Badge
                        variant="outline"
                        className={`text-[10px] capitalize font-mono ${
                          isRunning
                            ? 'bg-emerald-950/40 text-emerald-400 border-emerald-800/60'
                            : isCompleted
                            ? 'bg-blue-950/40 text-blue-300 border-blue-800/60'
                            : isPaused
                            ? 'bg-amber-950/40 text-amber-400 border-amber-800/60'
                            : 'bg-slate-900 text-slate-400 border-slate-800'
                        }`}
                      >
                        {isCompleted ? 'completed (0)' : container.state}
                      </Badge>
                    </td>

                    <td className="p-3.5">
                      {container.stack ? (
                        <div className="flex items-center gap-1.5">
                          <Badge variant="outline" className="text-[10px] text-blue-400 bg-blue-950/40 border-blue-800/50 font-mono flex items-center gap-1">
                            <Layers className="w-3 h-3" />
                            {container.stack}
                          </Badge>
                          {container.service && (
                            <span className="text-[10px] text-slate-500 font-mono">
                              ({container.service})
                            </span>
                          )}
                        </div>
                      ) : (
                        <Badge variant="outline" className="text-[10px] text-amber-400 bg-amber-950/40 border-amber-800/50 font-mono">
                          Standalone
                        </Badge>
                      )}
                    </td>

                    <td className="p-3.5 text-slate-400 max-w-[180px] truncate text-[11px]" title={container.image}>
                      {container.image}
                    </td>

                    <td className="p-3.5">
                      {container.ports && container.ports.length > 0 ? (
                        <div className="flex flex-wrap gap-1">
                          {container.ports.slice(0, 2).map((p, idx) => {
                            if (p.publicPort) {
                              const targetHost = getHostForContainer(container.hostEndpoint);
                              return (
                                <a
                                  key={idx}
                                  href={`http://${targetHost}:${p.publicPort}`}
                                  target="_blank"
                                  rel="noreferrer"
                                  onClick={(e) => e.stopPropagation()}
                                  title={`Open service at http://${targetHost}:${p.publicPort}`}
                                  className="inline-flex items-center text-[10px] font-mono tabular-nums px-1.5 py-0.5 rounded bg-slate-900 text-slate-300 border border-slate-800 hover:text-rose-400 hover:border-rose-500/40 hover:bg-slate-800 transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-rose-500"
                                >
                                  <span>{p.publicPort}:{p.privatePort}</span>
                                  <ExternalLink className="w-2.5 h-2.5 ml-1 opacity-70" />
                                </a>
                              );
                            }
                            return (
                              <span
                                key={idx}
                                className="text-[10px] font-mono tabular-nums px-1.5 py-0.5 rounded bg-slate-900 text-slate-300 border border-slate-800"
                              >
                                {p.privatePort}
                              </span>
                            );
                          })}
                          {container.ports.length > 2 && (
                            <span className="text-[10px] text-slate-500 font-mono tabular-nums">
                              +{container.ports.length - 2}
                            </span>
                          )}
                        </div>
                      ) : (
                        <span className="text-slate-600">—</span>
                      )}
                    </td>

                    <td className="p-3.5 text-right flex items-center justify-end gap-2">
                      <Button
                        size="sm"
                        variant="ghost"
                        onClick={(e) => {
                          e.stopPropagation();
                          setLogsForContainer(container);
                        }}
                        className="h-6 px-2 text-[10px] text-indigo-400 hover:text-white hover:bg-indigo-600/20 border border-indigo-500/20 font-sans"
                      >
                        <Terminal className="w-3 h-3 mr-1" />
                        Logs
                      </Button>
                      {isStandalone && (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={(e) => {
                            e.stopPropagation();
                            setGeneratingForContainer(container);
                          }}
                          className="h-6 px-2 text-[10px] text-amber-400 hover:text-white hover:bg-amber-600/20 border border-amber-500/20 font-sans"
                        >
                          <Sparkles className="w-3 h-3 mr-1" />
                          Manage
                        </Button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Slide-over Sheet for Container Detail */}
      <ContainerDetailSheet
        container={selectedContainer}
        isOpen={Boolean(selectedContainer)}
        onClose={() => setSelectedContainer(null)}
      />

      {/* Generate Compose Dialog */}
      <GenerateComposeDialog
        container={generatingForContainer}
        isOpen={Boolean(generatingForContainer)}
        onClose={() => setGeneratingForContainer(null)}
      />

      {/* Container Logs Modal */}
      {logsForContainer && (
        <ContainerLogsModal
          isOpen={Boolean(logsForContainer)}
          onClose={() => setLogsForContainer(null)}
          containerId={logsForContainer.id}
          containerName={logsForContainer.name}
          nodeName={logsForContainer.hostName}
          hostEndpoint={logsForContainer.hostEndpoint}
        />
      )}
    </div>
  );
};
