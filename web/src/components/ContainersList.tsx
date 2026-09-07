import React, { useState } from 'react';
import { ContainerSummary } from '../types';
import { 
  Box, 
  Search, 
  Filter, 
  ArrowRight, 
  Layers, 
  Clock, 
  Globe, 
  AlertCircle,
  RefreshCw,
  Shield
} from 'lucide-react';

interface ContainersListProps {
  containers: ContainerSummary[];
  loading: boolean;
  error?: string | null;
  onRefresh: () => void;
  selectedStack?: string | null;
  onClearStackFilter?: () => void;
}

export const ContainersList: React.FC<ContainersListProps> = ({
  containers,
  loading,
  error,
  onRefresh,
  selectedStack,
  onClearStackFilter,
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [stateFilter, setStateFilter] = useState<'all' | 'running' | 'exited' | 'paused'>('all');

  const filteredContainers = containers.filter((container) => {
    const matchesStack = !selectedStack || container.stack === selectedStack;
    const matchesState = stateFilter === 'all' || container.state.toLowerCase() === stateFilter;
    const term = searchTerm.toLowerCase();
    const matchesSearch = 
      container.name.toLowerCase().includes(term) ||
      container.image.toLowerCase().includes(term) ||
      (container.stack && container.stack.toLowerCase().includes(term)) ||
      (container.service && container.service.toLowerCase().includes(term)) ||
      container.id.toLowerCase().includes(term);

    return matchesStack && matchesState && matchesSearch;
  });

  const getStateBadgeClass = (state: string) => {
    switch (state.toLowerCase()) {
      case 'running':
        return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30';
      case 'exited':
      case 'dead':
        return 'bg-rose-500/10 text-rose-400 border-rose-500/30';
      case 'paused':
        return 'bg-amber-500/10 text-amber-400 border-amber-500/30';
      case 'restarting':
        return 'bg-blue-500/10 text-blue-400 border-blue-500/30';
      default:
        return 'bg-slate-800 text-slate-400 border-slate-700';
    }
  };

  return (
    <div className="space-y-6">
      {/* Controls & Active Stack Filter Banner */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-4">
        {/* Search */}
        <div className="relative flex-1 max-w-md">
          <Search className="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-slate-400" />
          <input
            type="text"
            placeholder="Search containers, images, ports..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-10 pr-4 py-2 bg-slate-900/80 border border-slate-800 rounded-xl text-sm text-slate-200 placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-rose-500/50 focus:border-transparent transition-all"
          />
        </div>

        {/* State filter buttons */}
        <div className="flex items-center gap-2 bg-slate-900/80 p-1 rounded-xl border border-slate-800">
          <Filter className="w-3.5 h-3.5 text-slate-400 ml-2 mr-1" />
          {(['all', 'running', 'exited', 'paused'] as const).map((st) => (
            <button
              key={st}
              onClick={() => setStateFilter(st)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium capitalize transition-all ${
                stateFilter === st
                  ? 'bg-rose-500 text-white shadow-sm shadow-rose-900/30'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
              }`}
            >
              {st}
            </button>
          ))}
        </div>
      </div>

      {/* Selected Stack Filter Pill (if coming from stack click) */}
      {selectedStack && (
        <div className="flex items-center gap-2 bg-slate-900/80 border border-slate-800 px-4 py-2 rounded-xl text-xs text-slate-300">
          <span>Filtering by stack:</span>
          <span className="font-semibold text-rose-400 font-mono bg-rose-500/10 px-2 py-0.5 rounded border border-rose-500/20">
            {selectedStack}
          </span>
          <button
            onClick={onClearStackFilter}
            className="ml-auto text-slate-400 hover:text-slate-200 text-xs underline"
          >
            Clear stack filter
          </button>
        </div>
      )}

      {/* Loading state */}
      {loading && containers.length === 0 && (
        <div className="space-y-3">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="bg-slate-900/60 border border-slate-800 rounded-xl p-4 animate-pulse h-20" />
          ))}
        </div>
      )}

      {/* Error state */}
      {error && (
        <div className="p-5 bg-rose-950/20 border border-rose-900/40 rounded-2xl flex items-center justify-between text-rose-300">
          <div className="flex items-center gap-3">
            <AlertCircle className="w-5 h-5 text-rose-400" />
            <div>
              <p className="font-semibold text-sm">Failed to load containers</p>
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

      {/* Container List */}
      {!loading && !error && filteredContainers.length === 0 ? (
        <div className="text-center py-16 px-4 bg-slate-900/40 border border-slate-800/60 rounded-2xl">
          <Box className="w-12 h-12 text-slate-600 mx-auto mb-3" />
          <h3 className="text-base font-semibold text-slate-300">No containers found</h3>
          <p className="text-xs text-slate-500 max-w-sm mx-auto mt-1">
            {searchTerm || stateFilter !== 'all' || selectedStack
              ? 'No containers match your search query or filters.'
              : 'No Docker containers currently exist on this node.'}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {filteredContainers.map((container) => {
            const shortId = container.id.substring(0, 12);
            const isRunning = container.state.toLowerCase() === 'running';
            const isSelf = Boolean(container.is_self);

            return (
              <div
                key={container.id}
                className={`rounded-xl p-4 transition-all duration-150 shadow-md ${
                  isSelf
                    ? 'bg-slate-900/85 hover:bg-slate-900 border border-purple-500/50 ring-1 ring-purple-500/30 shadow-purple-950/30 bg-gradient-to-r from-purple-950/20 via-slate-900/75 to-slate-900/70'
                    : 'bg-slate-900/70 hover:bg-slate-900 border border-slate-800 hover:border-slate-700/80'
                }`}
              >
                <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
                  {/* Left: Icon, Name, ID, Image, Stack */}
                  <div className="flex items-start gap-3.5 min-w-0">
                    <div
                      className={`p-2.5 rounded-xl flex-shrink-0 mt-0.5 ${
                        isSelf
                          ? 'bg-purple-500/20 text-purple-300 border border-purple-500/40 shadow-sm shadow-purple-950/40'
                          : 'bg-slate-800/80 text-slate-300'
                      }`}
                    >
                      {isSelf ? <Shield className="w-5 h-5 text-purple-400" /> : <Box className="w-5 h-5" />}
                    </div>
                    <div className="min-w-0">
                      <div className="flex items-center flex-wrap gap-2">
                        <span className="font-bold text-sm text-slate-100 font-mono truncate" title={container.name}>
                          {container.name.replace(/^\//, '')}
                        </span>

                        {/* Dokidoki Self badge */}
                        {isSelf && (
                          <span className="text-[10px] font-semibold bg-purple-500/20 text-purple-200 border border-purple-500/40 px-2 py-0.5 rounded-md flex items-center gap-1 shadow-sm shadow-purple-950/40">
                            <Shield className="w-3 h-3 text-purple-400" />
                            Dokidoki (Self)
                          </span>
                        )}

                        {/* State badge */}
                        <span
                          className={`text-[10px] font-bold uppercase px-2 py-0.5 rounded-md border flex items-center gap-1 ${getStateBadgeClass(
                            container.state
                          )}`}
                        >
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              isRunning ? 'bg-emerald-400 animate-pulse' : 'bg-rose-400'
                            }`}
                          />
                          {container.state}
                        </span>

                        {/* Stack / Service Badge */}
                        {container.stack && (
                          <span className="text-[10px] font-medium bg-purple-500/10 text-purple-300 border border-purple-500/20 px-2 py-0.5 rounded-md flex items-center gap-1">
                            <Layers className="w-3 h-3 text-purple-400" />
                            {container.stack} {container.service ? `(${container.service})` : ''}
                          </span>
                        )}
                      </div>

                      {/* Image & ID */}
                      <div className="flex items-center flex-wrap gap-3 mt-1.5 text-xs text-slate-400 font-mono">
                        <span className="text-slate-300 truncate max-w-xs" title={container.image}>
                          {container.image}
                        </span>
                        <span className="text-slate-600">•</span>
                        <span className="text-slate-500">{shortId}</span>
                        <span className="text-slate-600">•</span>
                        <span className="flex items-center gap-1 text-slate-400 text-[11px] font-sans">
                          <Clock className="w-3 h-3 text-slate-500" />
                          {container.status}
                        </span>
                      </div>
                    </div>
                  </div>

                  {/* Right: Port Mappings */}
                  <div className="flex items-center flex-wrap gap-2 lg:justify-end flex-shrink-0">
                    {container.ports && container.ports.length > 0 ? (
                      container.ports.map((port, idx) => {
                        const hasPublic = port.publicPort !== undefined && port.publicPort > 0;
                        return (
                          <div
                            key={idx}
                            className="flex items-center gap-1.5 bg-slate-950/80 border border-slate-800 px-2.5 py-1 rounded-lg text-xs font-mono text-slate-300"
                            title={`Port: ${port.ip || '0.0.0.0'}:${port.publicPort} -> ${port.privatePort}/${port.type}`}
                          >
                            <Globe className="w-3 h-3 text-slate-400" />
                            {hasPublic ? (
                              <>
                                <span className="text-rose-400 font-semibold">{port.publicPort}</span>
                                <ArrowRight className="w-3 h-3 text-slate-500" />
                                <span>{port.privatePort}</span>
                              </>
                            ) : (
                              <span>{port.privatePort}</span>
                            )}
                            <span className="text-[10px] text-slate-500 uppercase">{port.type}</span>
                          </div>
                        );
                      })
                    ) : (
                      <span className="text-xs text-slate-600 font-mono">No mapped ports</span>
                    )}
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
