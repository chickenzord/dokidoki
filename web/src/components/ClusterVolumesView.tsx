import React, { useState, useMemo, useDeferredValue } from 'react';
import { ClusterVolume } from '../types';
import { useClusterVolumesQuery } from '../hooks/useClusterData';
import { VolumeDetailSheet } from './VolumeDetailSheet';
import { Badge } from './ui/badge';
import { Input } from './ui/input';
import {
  HardDrive,
  Server,
  Search,
  AlertCircle,
  Loader2,
  ExternalLink,
  X,
  Globe,
  Folder,
} from 'lucide-react';

export interface ClusterVolumesViewProps {
  selectedHostId?: string | null;
  searchQuery?: string;
}

export const ClusterVolumesView: React.FC<ClusterVolumesViewProps> = ({
  selectedHostId,
  searchQuery: initialSearch = '',
}) => {
  const [localSearch, setLocalSearch] = useState('');
  const [driverFilter, setDriverFilter] = useState<string>('all');
  const [selectedVolume, setSelectedVolume] = useState<ClusterVolume | null>(null);

  const { data, isLoading, error } = useClusterVolumesQuery(selectedHostId);
  const volumes = data?.items ?? [];
  const unreachableNodes = data?.unreachableNodes ?? [];

  const rawSearch = initialSearch || localSearch;
  const deferredSearch = useDeferredValue(rawSearch);
  const activeSearch = deferredSearch.trim().toLowerCase();

  const drivers = useMemo(() => {
    const set = new Set<string>();
    volumes.forEach((v) => {
      if (v.Driver) set.add(v.Driver);
    });
    return Array.from(set).sort();
  }, [volumes]);

  const filteredVolumes = useMemo(() => {
    return volumes.filter((vol) => {
      if (driverFilter !== 'all' && vol.Driver !== driverFilter) return false;

      if (activeSearch) {
        const matchName = vol.Name.toLowerCase().includes(activeSearch);
        const matchDriver = (vol.Driver || '').toLowerCase().includes(activeSearch);
        const matchMount = (vol.Mountpoint || '').toLowerCase().includes(activeSearch);
        const matchHost = (vol.hostName || '').toLowerCase().includes(activeSearch);
        if (!matchName && !matchDriver && !matchMount && !matchHost) {
          return false;
        }
      }

      return true;
    });
  }, [volumes, driverFilter, activeSearch]);

  return (
    <div className="space-y-4">
      {/* Top Header Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <HardDrive className="w-5 h-5 text-slate-400" />
            Volumes
          </h2>
          <Badge variant="outline" className="text-xs font-mono tabular-nums bg-slate-900 border-slate-700 text-slate-300">
            {filteredVolumes.length} {filteredVolumes.length === 1 ? 'volume' : 'volumes'}
          </Badge>
        </div>

        {/* Search */}
        <div className="flex items-center gap-2">
          <div className="relative w-full sm:w-64">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <Input
              value={localSearch}
              onChange={(e) => setLocalSearch(e.target.value)}
              placeholder="Filter volumes..."
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
          <span>Loading volumes...</span>
        </div>
      ) : error ? (
        <div className="p-4 bg-rose-950/40 border border-rose-800/60 rounded-xl text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
          <span>Failed to load volumes: {(error as any)?.message || 'Unknown error'}</span>
        </div>
      ) : filteredVolumes.length === 0 ? (
        <div className="text-center py-16 border border-dashed border-slate-800 rounded-xl p-8">
          <HardDrive className="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">No volumes found</h3>
          <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
            {activeSearch || driverFilter !== 'all'
              ? 'No volumes match your filter criteria.'
              : 'There are no Docker volumes available on the selected node(s).'}
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
                <th className="py-2.5 px-3">Mountpoint</th>
                <th className="py-2.5 px-3">Created</th>
                <th className="py-2.5 px-3">Host Node</th>
                <th className="py-2.5 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-sans">
              {filteredVolumes.map((vol) => {
                const createdStr = vol.CreatedAt ? new Date(vol.CreatedAt).toLocaleDateString() : '-';

                return (
                  <tr
                    key={`${vol.hostId}-${vol.Name}`}
                    onClick={() => setSelectedVolume(vol)}
                    className="hover:bg-slate-800/40 transition-colors cursor-pointer group"
                  >
                    <td className="py-3 px-4 min-w-0">
                      <div className="flex items-center gap-2">
                        <HardDrive className="w-4 h-4 text-slate-400 flex-shrink-0" />
                        <span className="font-semibold text-slate-200 group-hover:text-white transition-colors truncate max-w-xs block">
                          {vol.Name}
                        </span>
                      </div>
                    </td>
                    <td className="py-3 px-3">
                      <Badge variant="outline" className="font-mono text-[10px] bg-slate-900 border-slate-700 text-slate-300">
                        {vol.Driver}
                      </Badge>
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-400">
                      <div className="flex items-center gap-1.5">
                        <Globe className="w-3 h-3 text-slate-400" />
                        <span>{vol.Scope || 'local'}</span>
                      </div>
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-400 truncate max-w-xs" title={vol.Mountpoint}>
                      <div className="flex items-center gap-1.5 truncate">
                        <Folder className="w-3 h-3 text-slate-500 flex-shrink-0" />
                        <span className="truncate">{vol.Mountpoint || '-'}</span>
                      </div>
                    </td>
                    <td className="py-3 px-3 text-slate-400 font-mono text-[11px]">
                      {createdStr}
                    </td>
                    <td className="py-3 px-3 text-slate-400">
                      <div className="flex items-center gap-1.5">
                        <Server className="w-3 h-3 text-slate-400" />
                        <span className="truncate">{vol.hostName}</span>
                      </div>
                    </td>
                    <td className="py-3 px-4 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedVolume(vol);
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

      {/* Volume Detail Sheet */}
      <VolumeDetailSheet
        volume={selectedVolume}
        isOpen={Boolean(selectedVolume)}
        onClose={() => setSelectedVolume(null)}
      />
    </div>
  );
};
