import React, { useState, useMemo, useDeferredValue } from 'react';
import { ClusterNetwork } from '../types';
import { useClusterNetworksQuery } from '../hooks/useClusterData';
import { NetworkDetailSheet } from './NetworkDetailSheet';
import { Badge } from './ui/badge';
import { Input } from './ui/input';
import {
  Network,
  Server,
  Search,
  AlertCircle,
  Loader2,
  ExternalLink,
  X,
  Globe,
} from 'lucide-react';

export interface ClusterNetworksViewProps {
  selectedHostId?: string | null;
  searchQuery?: string;
}

export const ClusterNetworksView: React.FC<ClusterNetworksViewProps> = ({
  selectedHostId,
  searchQuery: initialSearch = '',
}) => {
  const [localSearch, setLocalSearch] = useState('');
  const [driverFilter, setDriverFilter] = useState<string>('all');
  const [selectedNetwork, setSelectedNetwork] = useState<ClusterNetwork | null>(null);

  const { data, isLoading, error } = useClusterNetworksQuery(selectedHostId);
  const networks = data?.items ?? [];
  const unreachableNodes = data?.unreachableNodes ?? [];

  const rawSearch = initialSearch || localSearch;
  const deferredSearch = useDeferredValue(rawSearch);
  const activeSearch = deferredSearch.trim().toLowerCase();

  const drivers = useMemo(() => {
    const set = new Set<string>();
    networks.forEach((n) => {
      if (n.Driver) set.add(n.Driver);
    });
    return Array.from(set).sort();
  }, [networks]);

  const filteredNetworks = useMemo(() => {
    return networks.filter((net) => {
      if (driverFilter !== 'all' && net.Driver !== driverFilter) return false;

      if (activeSearch) {
        const netId = net.Id || net.ID || '';
        const matchName = net.Name.toLowerCase().includes(activeSearch);
        const matchId = netId.toLowerCase().includes(activeSearch);
        const matchDriver = (net.Driver || '').toLowerCase().includes(activeSearch);
        const matchHost = (net.hostName || '').toLowerCase().includes(activeSearch);
        if (!matchName && !matchId && !matchDriver && !matchHost) {
          return false;
        }
      }

      return true;
    });
  }, [networks, driverFilter, activeSearch]);

  return (
    <div className="space-y-4">
      {/* Top Header Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Network className="w-5 h-5 text-slate-400" />
            Networks
          </h2>
          <Badge variant="outline" className="text-xs font-mono tabular-nums bg-slate-900 border-slate-700 text-slate-300">
            {filteredNetworks.length} {filteredNetworks.length === 1 ? 'network' : 'networks'}
          </Badge>
        </div>

        {/* Search */}
        <div className="flex items-center gap-2">
          <div className="relative w-full sm:w-64">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <Input
              value={localSearch}
              onChange={(e) => setLocalSearch(e.target.value)}
              placeholder="Filter networks..."
              className="pl-9 pr-8 h-8 text-xs bg-slate-900/80 border-slate-700 text-slate-100 placeholder:text-slate-400"
            />
            {localSearch && (
              <button
                onClick={() => setLocalSearch('')}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-slate-400 hover:text-slate-200"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        </div>
      </div>

      {/* Driver Filters */}
      {drivers.length > 1 && (
        <div className="flex items-center gap-1.5 overflow-x-auto pb-1 text-xs">
          <span className="text-slate-400 font-medium mr-1">Driver:</span>
          <button
            onClick={() => setDriverFilter('all')}
            className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
              driverFilter === 'all'
                ? 'bg-slate-800 text-white border border-slate-700'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'
            }`}
          >
            All
          </button>
          {drivers.map((drv) => (
            <button
              key={drv}
              onClick={() => setDriverFilter(drv)}
              className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors font-mono ${
                driverFilter === drv
                  ? 'bg-slate-800 text-white border border-slate-700'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900'
              }`}
            >
              {drv}
            </button>
          ))}
        </div>
      )}

      {/* Unreachable Nodes Banner */}
      {unreachableNodes.length > 0 && (
        <div className="p-3 bg-amber-950/30 border border-amber-800/50 rounded-xl text-amber-300 text-xs flex items-start gap-2.5">
          <AlertCircle className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
          <div className="flex-1">
            <span className="font-semibold">Some nodes could not be reached:</span>
            <ul className="mt-1 list-disc list-inside space-y-0.5 text-amber-300/80">
              {unreachableNodes.map((n) => (
                <li key={n.id}>
                  {n.name}: {n.error || 'Connection failed'}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      {/* Content */}
      {isLoading ? (
        <div className="flex items-center justify-center py-20 text-slate-400">
          <Loader2 className="w-6 h-6 animate-spin text-slate-400 mr-2" />
          <span>Loading networks...</span>
        </div>
      ) : error ? (
        <div className="p-4 bg-rose-950/40 border border-rose-800/60 rounded-xl text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
          <span>Failed to load networks: {(error as any)?.message || 'Unknown error'}</span>
        </div>
      ) : filteredNetworks.length === 0 ? (
        <div className="text-center py-16 border border-dashed border-slate-800 rounded-xl p-8">
          <Network className="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">No networks found</h3>
          <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
            {activeSearch || driverFilter !== 'all'
              ? 'No networks match your filter criteria.'
              : 'There are no Docker networks available on the selected node(s).'}
          </p>
        </div>
      ) : (
        <div className="border border-slate-800/80 rounded-xl overflow-hidden bg-slate-900/20">
          <table className="w-full text-left text-xs border-collapse">
            <thead>
              <tr className="border-b border-slate-800 bg-slate-900/60 text-slate-400 font-medium">
                <th className="py-2.5 px-4">Name</th>
                <th className="py-2.5 px-3">Driver</th>
                <th className="py-2.5 px-3">Scope</th>
                <th className="py-2.5 px-3">Subnet / Gateway</th>
                <th className="py-2.5 px-3">Containers</th>
                <th className="py-2.5 px-3">Host Node</th>
                <th className="py-2.5 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-sans">
              {filteredNetworks.map((net) => {
                const subnets = net.IPAM?.Config?.map((c) => c.Subnet).filter(Boolean).join(', ') || '-';
                const containerCount = net.Containers ? Object.keys(net.Containers).length : 0;

                const netId = net.Id || net.ID || '';
                return (
                  <tr
                    key={`${net.hostId}-${netId}`}
                    onClick={() => setSelectedNetwork(net)}
                    className="hover:bg-slate-800/40 transition-colors cursor-pointer group"
                  >
                    <td className="py-3 px-4 min-w-0">
                      <div className="flex items-center gap-2">
                        <Network className="w-4 h-4 text-slate-400 flex-shrink-0" />
                        <div className="min-w-0">
                          <span className="font-semibold text-slate-200 group-hover:text-white transition-colors truncate block">
                            {net.Name}
                          </span>
                          <span className="text-[10px] font-mono text-slate-400 truncate block">
                            {netId.substring(0, 12)}
                          </span>
                        </div>
                      </div>
                    </td>
                    <td className="py-3 px-3">
                      <Badge variant="outline" className="font-mono text-[10px] bg-slate-900 border-slate-700 text-slate-300">
                        {net.Driver}
                      </Badge>
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-400">
                      <div className="flex items-center gap-1.5">
                        <Globe className="w-3 h-3 text-slate-400" />
                        <span>{net.Scope}</span>
                      </div>
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-300 truncate max-w-xs" title={subnets}>
                      {subnets}
                    </td>
                    <td className="py-3 px-3">
                      <Badge variant="outline" className="text-[10px] font-mono bg-slate-900 border-slate-700 text-slate-300">
                        {containerCount}
                      </Badge>
                    </td>
                    <td className="py-3 px-3 text-slate-400">
                      <div className="flex items-center gap-1.5">
                        <Server className="w-3 h-3 text-slate-400" />
                        <span className="truncate">{net.hostName}</span>
                      </div>
                    </td>
                    <td className="py-3 px-4 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedNetwork(net);
                        }}
                        className="inline-flex items-center gap-1 text-[11px] text-slate-400 hover:text-slate-200 transition-colors px-2 py-1 rounded bg-slate-900/60 border border-slate-800 hover:border-slate-700"
                      >
                        <span>Inspect</span>
                        <ExternalLink className="w-3 h-3" />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Network Detail Sheet */}
      <NetworkDetailSheet
        network={selectedNetwork}
        isOpen={Boolean(selectedNetwork)}
        onClose={() => setSelectedNetwork(null)}
      />
    </div>
  );
};
