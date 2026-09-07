import React, { useState } from 'react';
import { FleetStack, FleetContainer } from '../types';
import { useFleetStacksQuery } from '../hooks/useFleetData';
import { StackDetailSheet } from './StackDetailSheet';
import { ContainerDetailSheet } from './ContainerDetailSheet';
import { ImportStackDialog } from './ImportStackDialog';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Input } from './ui/input';
import {
  Layers,
  Server,
  Download,
  Search,
  AlertCircle,
  Loader2,
} from 'lucide-react';

interface FleetStacksViewProps {
  selectedHostId?: string | null;
  searchQuery?: string;
}

export const FleetStacksView: React.FC<FleetStacksViewProps> = ({
  selectedHostId,
  searchQuery: initialSearch = '',
}) => {
  const [sourceFilter, setSourceFilter] = useState<'all' | 'managed' | 'external'>('all');
  const [statusFilter, setStatusFilter] = useState<'all' | 'running'>('all');
  const [localSearch, setLocalSearch] = useState('');
  const [selectedStack, setSelectedStack] = useState<FleetStack | null>(null);
  const [importingStack, setImportingStack] = useState<FleetStack | null>(null);

  // For opening a container from the stack sheet
  const [selectedContainer, setSelectedContainer] = useState<FleetContainer | null>(null);

  const { data: stacks = [], isLoading, error } = useFleetStacksQuery(selectedHostId);

  const activeSearch = (initialSearch || localSearch).trim().toLowerCase();

  const filteredStacks = stacks.filter((stack) => {
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

  const totalRunningContainers = stacks.reduce((sum, s) => sum + s.rollup.running, 0);

  return (
    <div className="space-y-4">
      {/* Top Controls Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-2 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Layers className="w-5 h-5 text-rose-500" />
            Compose Stacks
          </h2>
          <Badge variant="outline" className="text-xs font-mono bg-slate-900 border-slate-700 text-slate-300">
            {filteredStacks.length} {filteredStacks.length === 1 ? 'stack' : 'stacks'} • {totalRunningContainers} running
          </Badge>
        </div>

        {/* Filters */}
        <div className="flex flex-wrap items-center gap-2">
          {/* Source Filter Pills */}
          <div className="flex items-center rounded-lg bg-slate-900 border border-slate-800 p-0.5 text-xs">
            <button
              onClick={() => setSourceFilter('all')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                sourceFilter === 'all'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              All Sources
            </button>
            <button
              onClick={() => setSourceFilter('managed')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                sourceFilter === 'managed'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Managed
            </button>
            <button
              onClick={() => setSourceFilter('external')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                sourceFilter === 'external'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              External
            </button>
          </div>

          {/* Running vs All */}
          <div className="flex items-center rounded-lg bg-slate-900 border border-slate-800 p-0.5 text-xs">
            <button
              onClick={() => setStatusFilter('all')}
              className={`px-2.5 py-1 rounded-md transition-all ${
                statusFilter === 'all'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              All
            </button>
            <button
              onClick={() => setStatusFilter('running')}
              className={`px-2.5 py-1 rounded-md transition-all flex items-center gap-1.5 ${
                statusFilter === 'running'
                  ? 'bg-slate-800 text-white font-medium shadow-sm'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
              Running Only
            </button>
          </div>

          {/* Local search if navbar search is empty */}
          {!initialSearch && (
            <div className="relative">
              <Search className="w-3.5 h-3.5 text-slate-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
              <Input
                value={localSearch}
                onChange={(e) => setLocalSearch(e.target.value)}
                placeholder="Filter stacks..."
                className="h-8 pl-8 pr-2.5 text-xs bg-slate-900 border-slate-800 w-36 sm:w-44 focus:ring-rose-500 text-slate-200"
              />
            </div>
          )}
        </div>
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
          <span>Scanning fleet stacks...</span>
        </div>
      ) : filteredStacks.length === 0 ? (
        <div className="p-12 border border-slate-800 rounded-xl text-center text-xs text-slate-500 bg-slate-950/40">
          No stacks found matching your filters.
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-3.5">
          {filteredStacks.map((stack) => {
            const isManaged = stack.source === 'managed';
            const hasRunning = stack.rollup.running > 0;
            const isAllRunning = stack.rollup.total > 0 && stack.rollup.running === stack.rollup.total;

            return (
              <div
                key={`${stack.hostId}-${stack.name}`}
                onClick={() => setSelectedStack(stack)}
                className="group p-4 rounded-xl border border-slate-800/80 bg-slate-900/60 hover:bg-slate-900/90 hover:border-slate-700 transition-all cursor-pointer flex flex-col justify-between gap-3 relative shadow-sm"
              >
                {/* Header: Title & Badges */}
                <div className="space-y-2">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex items-center gap-2 min-w-0">
                      <Layers className="w-4 h-4 text-rose-400 flex-shrink-0 mt-0.5" />
                      <h3 className="font-bold text-sm text-slate-100 group-hover:text-white truncate">
                        {stack.name}
                      </h3>
                    </div>

                    <div className="flex items-center gap-1.5 flex-shrink-0">
                      {isManaged ? (
                        <Badge variant="outline" className="text-[10px] bg-emerald-950/40 text-emerald-400 border-emerald-800/60 font-mono">
                          Managed
                        </Badge>
                      ) : (
                        <Badge variant="outline" className="text-[10px] bg-amber-950/40 text-amber-400 border-amber-800/60 font-mono">
                          External
                        </Badge>
                      )}
                    </div>
                  </div>

                  {/* Subdued Host Badge & Path */}
                  <div className="flex flex-wrap items-center gap-1.5 text-xs text-slate-400">
                    <Badge variant="outline" className="text-[10px] bg-slate-950 text-slate-300 border-slate-800 font-mono flex items-center gap-1">
                      <Server className="w-3 h-3 text-slate-500" />
                      {stack.hostName}
                    </Badge>

                    {stack.composePath && (
                      <span className="text-[10px] text-slate-500 font-mono truncate max-w-[180px]" title={stack.composePath}>
                        {stack.composePath.split('/').pop()}
                      </span>
                    )}
                  </div>
                </div>

                {/* Services Pills */}
                {stack.services && stack.services.length > 0 && (
                  <div className="flex flex-wrap gap-1">
                    {stack.services.slice(0, 4).map((svc) => (
                      <span
                        key={svc}
                        className="text-[10px] font-mono px-2 py-0.5 rounded bg-slate-950/80 text-slate-400 border border-slate-800/80 truncate max-w-[120px]"
                      >
                        {svc}
                      </span>
                    ))}
                    {stack.services.length > 4 && (
                      <span className="text-[10px] text-slate-500 font-mono px-1">
                        +{stack.services.length - 4}
                      </span>
                    )}
                  </div>
                )}

                {/* Footer: Container Rollups & Actions */}
                <div className="pt-2 border-t border-slate-800/80 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2 text-xs font-mono">
                    <span
                      className={`w-2 h-2 rounded-full ${
                        isAllRunning
                          ? 'bg-emerald-400 ring-2 ring-emerald-400/20'
                          : hasRunning
                          ? 'bg-amber-400'
                          : 'bg-slate-600'
                      }`}
                    />
                    <span className="text-slate-300 font-semibold">
                      {stack.rollup.running} / {stack.rollup.total}
                    </span>
                    <span className="text-slate-500 text-[11px]">containers</span>
                  </div>

                  {!isManaged && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={(e) => {
                        e.stopPropagation();
                        setImportingStack(stack);
                      }}
                      className="h-6 px-2 text-[11px] text-rose-400 hover:text-white hover:bg-rose-600/20 border border-rose-500/20"
                    >
                      <Download className="w-3 h-3 mr-1" />
                      Import
                    </Button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Slide-over Sheet for Stack Detail */}
      <StackDetailSheet
        stack={selectedStack}
        isOpen={Boolean(selectedStack)}
        onClose={() => setSelectedStack(null)}
        onSelectContainer={(containerId, hostEndpoint) => {
          setSelectedContainer({
            id: containerId,
            name: containerId.substring(0, 12),
            image: '',
            state: 'running',
            status: '',
            created: 0,
            ports: [],
            labels: {},
            hostId: '',
            hostName: selectedStack?.hostName || '',
            hostEndpoint,
          });
        }}
      />

      {/* Slide-over Sheet for Container Detail if opened from Stack */}
      <ContainerDetailSheet
        container={selectedContainer}
        isOpen={Boolean(selectedContainer)}
        onClose={() => setSelectedContainer(null)}
      />

      {/* Import External Stack Dialog */}
      <ImportStackDialog
        stack={importingStack}
        isOpen={Boolean(importingStack)}
        onClose={() => setImportingStack(null)}
      />
    </div>
  );
};
