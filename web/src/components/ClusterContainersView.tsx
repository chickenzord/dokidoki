import React, { useState, useMemo, useDeferredValue } from 'react';
import { useRouter } from '@tanstack/react-router';
import { ClusterContainer } from '../types';
import { useClusterContainersQuery } from '../hooks/useClusterData';
import { getHostForContainer } from '../lib/utils';
import { GenerateComposeDialog } from './GenerateComposeDialog';
import { ContainerLogsModal } from './ContainerLogsModal';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { FilterPills } from './ui/FilterPills';
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
  const router = useRouter();
  const [stateFilter, setStateFilter] = useState<'all' | 'running' | 'exited' | 'paused'>('all');
  const [typeFilter, setTypeFilter] = useState<'all' | 'standalone' | 'stack'>('all');
  const [localSearch, setLocalSearch] = useState('');
  const [generatingForContainer, setGeneratingForContainer] = useState<ClusterContainer | null>(null);
  const [logsForContainer, setLogsForContainer] = useState<ClusterContainer | null>(null);

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
      {/* Top Header Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Box className="w-5 h-5 text-slate-400" />
            Containers
          </h2>
          <Badge variant="outline" className="text-xs font-mono tabular-nums bg-slate-900 border-slate-700 text-slate-300">
            {runningCount} running • {filteredContainers.length} total
          </Badge>
        </div>

        {/* Search */}
        {!initialSearch && (
          <div className="relative w-full sm:w-64">
            <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <Input
              value={localSearch}
              onChange={(e) => setLocalSearch(e.target.value)}
              placeholder="Filter containers…"
              className="h-8 pl-8 pr-7 text-xs bg-slate-900 border-slate-800 w-full focus-visible:ring-1 focus-visible:ring-slate-700 text-slate-200 placeholder:text-slate-500 rounded-lg"
            />
            {localSearch && (
              <button
                onClick={() => setLocalSearch('')}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-slate-400 hover:text-white p-0.5"
                aria-label="Clear search"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        )}
      </div>

      {/* Line 2: Filter Group Bar */}
      <div className="flex flex-wrap items-center justify-between gap-2.5 pt-0.5">
        <div className="flex flex-wrap items-center gap-2">
          {/* State Filter Pills */}
          <FilterPills
            value={stateFilter}
            onChange={setStateFilter}
            options={[
              { value: 'all', label: 'All States' },
              { value: 'running', label: 'Running', dotColor: 'bg-emerald-400' },
              { value: 'exited', label: 'Exited' },
              { value: 'paused', label: 'Paused' },
            ]}
          />

          {/* Type Filter Pills */}
          <FilterPills
            value={typeFilter}
            onChange={setTypeFilter}
            options={[
              { value: 'all', label: 'All' },
              { value: 'stack', label: 'In Stack' },
              { value: 'standalone', label: 'Standalone' },
            ]}
          />
        </div>

        {/* Unreachable node footnote if any */}
        {unreachableNodes.length > 0 && (
          <div
            className="flex items-center gap-1.5 text-[11px] text-amber-400/90 font-mono"
            title={unreachableNodes.map((n) => `${n.name}: ${n.error || 'unreachable'}`).join('\n')}
          >
            <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
            <span>{unreachableNodes.length} node{unreachableNodes.length > 1 ? 's' : ''} unreachable</span>
          </div>
        )}
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
                    onClick={() => router.navigate({ to: `/nodes/${encodeURIComponent(container.hostId)}/containers/${encodeURIComponent(container.id)}` })}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        router.navigate({ to: `/nodes/${encodeURIComponent(container.hostId)}/containers/${encodeURIComponent(container.id)}` });
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
                              const targetHost = getHostForContainer(container.hostEndpoint, container.hostAddress);
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
                        className="h-6 px-2 text-[10px] text-slate-300 hover:text-white hover:bg-slate-800 border border-slate-700/60 font-sans"
                      >
                        <Terminal className="w-3 h-3 mr-1 text-slate-400" />
                        Logs
                      </Button>
                      {isStandalone && !container.hostReadOnly && (
                        <Button
                          size="sm"
                          variant="ghost"
                          onClick={(e) => {
                            e.stopPropagation();
                            setGeneratingForContainer(container);
                          }}
                          className="h-6 px-2 text-[10px] text-slate-300 hover:text-white hover:bg-slate-800 border border-slate-700/60 font-sans"
                        >
                          <Sparkles className="w-3 h-3 mr-1 text-slate-400" />
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
