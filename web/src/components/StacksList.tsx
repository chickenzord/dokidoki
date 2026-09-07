import React, { useState } from 'react';
import { StackSummary, ContainerSummary } from '../types';
import { 
  Layers, 
  FileCode2, 
  AlertCircle, 
  Search, 
  Filter, 
  RefreshCw,
  FolderKanban,
  Shield
} from 'lucide-react';

interface StacksListProps {
  stacks: StackSummary[];
  containers?: ContainerSummary[];
  loading: boolean;
  error?: string | null;
  onRefresh: () => void;
  onSelectStack?: (stackName: string) => void;
}

export const StacksList: React.FC<StacksListProps> = ({
  stacks,
  containers = [],
  loading,
  error,
  onRefresh,
  onSelectStack,
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [sourceFilter, setSourceFilter] = useState<'all' | 'managed' | 'external'>('all');

  const filteredStacks = stacks.filter((stack) => {
    const matchesSearch = stack.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (stack.services && stack.services.some(s => s.toLowerCase().includes(searchTerm.toLowerCase())));
    const matchesSource = sourceFilter === 'all' || stack.source === sourceFilter;
    return matchesSearch && matchesSource;
  });

  return (
    <div className="space-y-6">
      {/* Controls: Search and Filters */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4">
        {/* Search */}
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
          <input
            type="text"
            placeholder="Search stacks or services..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 bg-slate-900/80 border border-slate-800 rounded-xl text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-rose-500/50 focus:border-transparent transition-all"
          />
        </div>

        {/* Source Filter Buttons */}
        <div className="flex items-center gap-2 bg-slate-900/80 p-1 rounded-xl border border-slate-800">
          <Filter className="w-3.5 h-3.5 text-slate-400 ml-2 mr-1" />
          {(['all', 'managed', 'external'] as const).map((source) => (
            <button
              key={source}
              onClick={() => setSourceFilter(source)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium capitalize transition-all ${
                sourceFilter === source
                  ? 'bg-rose-500 text-white shadow-sm shadow-rose-900/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
              }`}
            >
              {source}
            </button>
          ))}
        </div>
      </div>

      {/* Loading state */}
      {loading && stacks.length === 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {[1, 2, 3].map((i) => (
            <div key={i} className="bg-slate-900/60 border border-slate-800 rounded-2xl p-5 animate-pulse h-48" />
          ))}
        </div>
      )}

      {/* Error state */}
      {error && (
        <div className="p-5 bg-rose-950/20 border border-rose-900/40 rounded-2xl flex items-center justify-between text-rose-300">
          <div className="flex items-center gap-3">
            <AlertCircle className="w-5 h-5 text-rose-400" />
            <div>
              <p className="font-semibold text-sm">Failed to load stacks</p>
              <p className="text-xs text-rose-400/80">{error}</p>
            </div>
          </div>
          <button
            onClick={onRefresh}
            className="px-3 py-1.5 rounded-lg bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 text-xs font-medium flex items-center gap-1.5 transition-colors"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            Retry
          </button>
        </div>
      )}

      {/* Stacks Grid */}
      {!loading && !error && filteredStacks.length === 0 ? (
        <div className="text-center py-16 px-4 bg-slate-900/40 border border-slate-800/60 rounded-2xl">
          <FolderKanban className="w-12 h-12 text-slate-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-slate-300">No stacks found</h3>
          <p className="text-xs text-slate-500 max-w-sm mx-auto mt-1">
            {searchTerm || sourceFilter !== 'all'
              ? 'No compose stacks match your search query or active filter.'
              : 'No compose stacks are currently managed or detected on this node.'}
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {filteredStacks.map((stack) => {
            const isManaged = stack.source === 'managed';
            const totalContainers = stack.rollup.total;
            const runningContainers = stack.rollup.running;
            const isFullyRunning = totalContainers > 0 && runningContainers === totalContainers;
            const isPartiallyRunning = runningContainers > 0 && runningContainers < totalContainers;
            const isSelfStack =
              Boolean(stack.is_self) ||
              stack.name.toLowerCase().includes('dokidoki') ||
              containers.some(
                (c) => c.is_self && (c.stack === stack.name || (!c.stack && stack.name.toLowerCase().includes('dokidoki')))
              );

            return (
              <div
                key={stack.name}
                onClick={() => onSelectStack?.(stack.name)}
                className={`group rounded-2xl p-5 shadow-lg transition-all duration-200 flex flex-col justify-between hover:shadow-xl hover:shadow-black/40 cursor-pointer ${
                  isSelfStack
                    ? 'bg-slate-900/85 hover:bg-slate-900 border border-purple-500/50 ring-1 ring-purple-500/30 shadow-purple-950/30 bg-gradient-to-b from-purple-950/20 via-slate-900/80 to-slate-900/70'
                    : 'bg-slate-900/70 hover:bg-slate-900 border border-slate-800 hover:border-slate-700/90'
                }`}
              >
                <div>
                  {/* Top Bar: Title & Source */}
                  <div className="flex items-start justify-between gap-2 mb-3">
                    <div className="min-w-0 flex items-center gap-2.5">
                      <div
                        className={`p-2 rounded-xl transition-colors ${
                          isSelfStack
                            ? 'bg-purple-500/20 text-purple-300 border border-purple-500/40 shadow-sm shadow-purple-950/30'
                            : 'bg-slate-800/80 group-hover:bg-rose-500/10 group-hover:text-rose-400 text-slate-400'
                        }`}
                      >
                        {isSelfStack ? <Shield className="w-5 h-5 text-purple-400" /> : <Layers className="w-5 h-5" />}
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <h3 className="text-base font-bold text-slate-100 group-hover:text-white truncate" title={stack.name}>
                            {stack.name}
                          </h3>
                          {isSelfStack && (
                            <span className="text-[10px] font-semibold bg-purple-500/20 text-purple-200 border border-purple-500/40 px-2 py-0.5 rounded-md flex items-center gap-1 shadow-sm shadow-purple-950/40">
                              <Shield className="w-3 h-3 text-purple-400" />
                              Self
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-1.5 mt-0.5">
                          {/* Source badge */}
                          <span
                            className={`text-[10px] font-semibold uppercase px-2 py-0.5 rounded-md border ${
                              isManaged
                                ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                                : 'bg-indigo-500/10 text-indigo-400 border-indigo-500/20'
                            }`}
                          >
                            {stack.source}
                          </span>

                          {/* Compose presence badge */}
                          <span
                            className={`text-[10px] font-semibold px-2 py-0.5 rounded-md border flex items-center gap-1 ${
                              stack.composePresent
                                ? 'bg-cyan-500/10 text-cyan-400 border-cyan-500/20'
                                : 'bg-slate-800 text-slate-400 border-slate-700'
                            }`}
                            title={stack.composePath || 'No compose file path'}
                          >
                            <FileCode2 className="w-3 h-3" />
                            {stack.composePresent ? 'Compose file' : 'Detached'}
                          </span>
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Services pills */}
                  {stack.services && stack.services.length > 0 && (
                    <div className="mb-4">
                      <span className="text-[11px] font-medium text-slate-400 block mb-1.5">
                        Services ({stack.services.length}):
                      </span>
                      <div className="flex flex-wrap gap-1.5 max-h-16 overflow-hidden">
                        {stack.services.map((svc) => (
                          <span
                            key={svc}
                            className="text-[11px] bg-slate-950/80 text-slate-300 px-2 py-0.5 rounded-md border border-slate-800/80 font-mono"
                          >
                            {svc}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                </div>

                {/* Bottom: Container Rollups */}
                <div className="pt-4 border-t border-slate-800/80 mt-2">
                  <div className="flex items-center justify-between text-xs mb-2">
                    <span className="text-slate-400 font-medium">Containers</span>
                    <span className="font-mono text-slate-300">
                      <strong className={isFullyRunning ? 'text-emerald-400' : isPartiallyRunning ? 'text-amber-400' : 'text-slate-400'}>
                        {stack.rollup.running}
                      </strong>{' '}
                      / {stack.rollup.total} running
                    </span>
                  </div>

                  {/* Status pills */}
                  <div className="grid grid-cols-4 gap-1.5 text-[11px] text-center font-mono">
                    <div className={`py-1 rounded bg-slate-950 border ${stack.rollup.running > 0 ? 'text-emerald-400 border-emerald-900/50' : 'text-slate-500 border-slate-800'}`}>
                      <span className="block text-[9px] uppercase tracking-wider text-slate-500">Run</span>
                      {stack.rollup.running}
                    </div>
                    <div className={`py-1 rounded bg-slate-950 border ${stack.rollup.exited > 0 ? 'text-rose-400 border-rose-900/50' : 'text-slate-500 border-slate-800'}`}>
                      <span className="block text-[9px] uppercase tracking-wider text-slate-500">Exit</span>
                      {stack.rollup.exited}
                    </div>
                    <div className={`py-1 rounded bg-slate-950 border ${stack.rollup.paused > 0 ? 'text-amber-400 border-amber-900/50' : 'text-slate-500 border-slate-800'}`}>
                      <span className="block text-[9px] uppercase tracking-wider text-slate-500">Pause</span>
                      {stack.rollup.paused}
                    </div>
                    <div className={`py-1 rounded bg-slate-950 border ${stack.rollup.restarting > 0 ? 'text-blue-400 border-blue-900/50' : 'text-slate-500 border-slate-800'}`}>
                      <span className="block text-[9px] uppercase tracking-wider text-slate-500">Rest</span>
                      {stack.rollup.restarting}
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
