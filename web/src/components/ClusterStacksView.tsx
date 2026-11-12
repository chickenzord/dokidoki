import React, { useState, useMemo, useDeferredValue } from 'react';
import { useRouter } from '@tanstack/react-router';
import { useClusterStacksQuery } from '../hooks/useClusterData';
import { Badge } from './ui/badge';
import { Input } from './ui/input';
import { FilterPills } from './ui/FilterPills';
import {
  Layers,
  Search,
  AlertCircle,
  AlertTriangle,
  HelpCircle,
  Loader2,
  X,
} from 'lucide-react';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from './ui/tooltip';

export interface ClusterStacksViewProps {
  selectedHostId?: string | null;
  searchQuery?: string;
}

export const ClusterStacksView: React.FC<ClusterStacksViewProps> = ({
  selectedHostId,
  searchQuery: initialSearch = '',
}) => {
  const router = useRouter();
  const [sourceFilter, setSourceFilter] = useState<'all' | 'managed' | 'external'>('all');
  const [statusFilter, setStatusFilter] = useState<'all' | 'running'>('all');
  const [localSearch, setLocalSearch] = useState('');

  const { data, isLoading, error } = useClusterStacksQuery(selectedHostId);
  const stacks = data?.items ?? [];
  const unreachableNodes = data?.unreachableNodes ?? [];

  const rawSearch = initialSearch || localSearch;
  const deferredSearch = useDeferredValue(rawSearch);
  const activeSearch = deferredSearch.trim().toLowerCase();

  const filteredStacks = useMemo(() => {
    return stacks.filter((stack) => {
      // Source filter
      if (sourceFilter !== 'all' && stack.source !== sourceFilter) {
        return false;
      }

      // Status filter
      if (statusFilter === 'running' && stack.rollup.running === 0) {
        return false;
      }

      // Search query filter
      if (activeSearch) {
        const matchName = stack.name.toLowerCase().includes(activeSearch);
        const matchHost = stack.hostName.toLowerCase().includes(activeSearch);
        const matchService = stack.services.some((s) => s.toLowerCase().includes(activeSearch));
        if (!matchName && !matchHost && !matchService) {
          return false;
        }
      }

      return true;
    });
  }, [stacks, sourceFilter, statusFilter, activeSearch]);

  const totalRunningContainers = useMemo(
    () => stacks.reduce((sum, s) => sum + s.rollup.running, 0),
    [stacks]
  );

  return (
    <div className="space-y-4">
      {/* Top Header Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Layers className="w-5 h-5 text-slate-400" />
            Compose Stacks
          </h2>
          <Badge variant="outline" className="text-xs font-mono tabular-nums bg-slate-900 border-slate-700 text-slate-300">
            {filteredStacks.length} {filteredStacks.length === 1 ? 'stack' : 'stacks'} • {totalRunningContainers} running
          </Badge>
        </div>

        {/* Search */}
        {!initialSearch && (
          <div className="relative w-full sm:w-64">
            <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <Input
              value={localSearch}
              onChange={(e) => setLocalSearch(e.target.value)}
              placeholder="Filter stacks…"
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
          {/* Source Filter Pills */}
          <FilterPills
            value={sourceFilter}
            onChange={setSourceFilter}
            options={[
              { value: 'all', label: 'All' },
              { value: 'managed', label: 'Managed' },
              { value: 'external', label: 'External' },
            ]}
          />

          {/* Status Filter Pills */}
          <FilterPills
            value={statusFilter}
            onChange={setStatusFilter}
            options={[
              { value: 'all', label: 'All' },
              { value: 'running', label: 'Running Only', dotColor: 'bg-emerald-400' },
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
          <span>Failed to load stacks: {(error as Error).message}</span>
        </div>
      )}

      {isLoading ? (
        <div className="p-16 text-center text-xs text-slate-400 flex flex-col items-center justify-center gap-2">
          <Loader2 className="w-6 h-6 animate-spin text-rose-500" />
          <span>Loading stacks…</span>
        </div>
      ) : filteredStacks.length === 0 ? (
        <div className="p-12 border border-slate-800 rounded-xl text-center text-xs text-slate-500 bg-slate-950/40">
          No stacks found matching your filters.
        </div>
      ) : (
        <TooltipProvider delayDuration={150}>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
            {filteredStacks.map((stack) => {
              const runningCount = stack.rollup.running;
              const completedCount = stack.rollup.completed || 0;
              const totalCount = stack.rollup.total;
              const hasRunning = runningCount > 0;
              const isAllHealthy = totalCount > 0 && (runningCount + completedCount) === totalCount;
              const isAllCompleted = totalCount > 0 && completedCount === totalCount;

              let statusColor = 'bg-slate-600';
              let statusTitle = 'Stopped';
              if (isAllHealthy) {
                statusColor = 'bg-emerald-400';
                statusTitle = isAllCompleted
                  ? 'Completed'
                  : completedCount > 0
                  ? `Running (${completedCount} completed)`
                  : 'Running';
              } else if (hasRunning) {
                statusColor = 'bg-amber-400';
                statusTitle = 'Partially Running';
              }

              return (
                <div
                  key={`${stack.hostId}-${stack.name}`}
                  role="button"
                  tabIndex={0}
                  onClick={() => router.navigate({ to: `/nodes/${encodeURIComponent(stack.hostId)}/stacks/${encodeURIComponent(stack.name)}` })}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      router.navigate({ to: `/nodes/${encodeURIComponent(stack.hostId)}/stacks/${encodeURIComponent(stack.name)}` });
                    }
                  }}
                  className="group px-3.5 py-3 rounded-lg border border-slate-800/80 bg-slate-900/40 hover:bg-slate-900 hover:border-slate-700 transition-colors cursor-pointer flex items-center justify-between gap-3 shadow-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-rose-500"
                >
                  <div className="flex items-center gap-2.5 min-w-0">
                    <span
                      className={`w-2 h-2 rounded-full flex-shrink-0 ${statusColor}`}
                      title={statusTitle}
                    />
                    <span className="font-medium text-sm text-slate-200 group-hover:text-white truncate">
                      {stack.name}
                    </span>
                  </div>

                  <div className="flex items-center gap-2 flex-shrink-0">
                    {/* Management status icon indicators */}
                    {stack.source === 'managed' && stack.pending_import ? (
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span
                            onClick={(e) => e.stopPropagation()}
                            className="p-1 rounded text-amber-400 hover:text-amber-300 hover:bg-amber-950/40 transition-colors cursor-help inline-flex items-center"
                            aria-label="Import Pending"
                          >
                            <AlertTriangle className="w-3.5 h-3.5" />
                          </span>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="bg-slate-900 text-slate-200 border border-slate-700 max-w-xs text-xs p-2.5">
                          <p className="font-semibold text-amber-300">Import Pending</p>
                          <p className="text-[11px] text-slate-300 mt-1 leading-relaxed">
                            Configuration imported, but containers are still running from the external directory. Run &apos;Up&apos; to redeploy.
                          </p>
                        </TooltipContent>
                      </Tooltip>
                    ) : stack.source === 'external' ? (
                      <Tooltip>
                        <TooltipTrigger asChild>
                          <span
                            onClick={(e) => e.stopPropagation()}
                            className="p-1 rounded text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 transition-colors cursor-help inline-flex items-center"
                            aria-label="Unmanaged Stack"
                          >
                            <HelpCircle className="w-3.5 h-3.5" />
                          </span>
                        </TooltipTrigger>
                        <TooltipContent side="top" className="bg-slate-900 text-slate-200 border border-slate-700 max-w-xs text-xs p-2.5">
                          <p className="font-semibold text-slate-200">Unmanaged Stack</p>
                          <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
                            Running externally on host. Open to view configuration and import into Dokidoki.
                          </p>
                        </TooltipContent>
                      </Tooltip>
                    ) : null}

                    <Badge
                      variant="outline"
                      className="text-[11px] font-mono text-slate-400 border-slate-800 bg-slate-950/60"
                    >
                      {stack.hostName}
                    </Badge>
                  </div>
                </div>
              );
            })}
          </div>
        </TooltipProvider>
      )}
    </div>
  );
};
