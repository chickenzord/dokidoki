import React, { useState, useMemo, useDeferredValue } from 'react';
import { ClusterImage } from '../types';
import { useClusterImagesQuery } from '../hooks/useClusterData';
import { ImageDetailSheet } from './ImageDetailSheet';
import { Badge } from './ui/badge';
import { Input } from './ui/input';
import {
  Layers,
  Server,
  Search,
  AlertCircle,
  Loader2,
  ExternalLink,
  X,
  HardDrive,
  Box,
} from 'lucide-react';

export interface ClusterImagesViewProps {
  selectedHostId?: string | null;
  searchQuery?: string;
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}

export const ClusterImagesView: React.FC<ClusterImagesViewProps> = ({
  selectedHostId,
  searchQuery: initialSearch = '',
}) => {
  const [localSearch, setLocalSearch] = useState('');
  const [selectedImage, setSelectedImage] = useState<ClusterImage | null>(null);

  const { data, isLoading, error } = useClusterImagesQuery(selectedHostId);
  const images = data?.items ?? [];
  const unreachableNodes = data?.unreachableNodes ?? [];

  const rawSearch = initialSearch || localSearch;
  const deferredSearch = useDeferredValue(rawSearch);
  const activeSearch = deferredSearch.trim().toLowerCase();

  const filteredImages = useMemo(() => {
    return images.filter((img) => {
      if (activeSearch) {
        const imgId = img.Id || img.ID || '';
        const matchTags = (img.RepoTags || []).some((t) => t.toLowerCase().includes(activeSearch));
        const matchId = imgId.toLowerCase().includes(activeSearch);
        const matchHost = (img.hostName || '').toLowerCase().includes(activeSearch);
        if (!matchTags && !matchId && !matchHost) {
          return false;
        }
      }

      return true;
    });
  }, [images, activeSearch]);

  const totalSize = useMemo(() => {
    return images.reduce((acc, img) => acc + (img.Size || 0), 0);
  }, [images]);

  return (
    <div className="space-y-4">
      {/* Top Header Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
        <div className="flex items-center gap-2">
          <h2 className="text-lg font-bold text-slate-100 flex items-center gap-2">
            <Layers className="w-5 h-5 text-slate-400" />
            Images
          </h2>
          <Badge variant="outline" className="text-xs font-mono tabular-nums bg-slate-900 border-slate-700 text-slate-300">
            {filteredImages.length} {filteredImages.length === 1 ? 'image' : 'images'} • {formatBytes(totalSize)}
          </Badge>
        </div>

        {/* Search */}
        <div className="flex items-center gap-2">
          <div className="relative w-full sm:w-64">
            <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
            <Input
              value={localSearch}
              onChange={(e) => setLocalSearch(e.target.value)}
              placeholder="Filter images..."
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
          <span>Loading images...</span>
        </div>
      ) : error ? (
        <div className="p-4 bg-rose-950/40 border border-rose-800/60 rounded-xl text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
          <span>Failed to load images: {(error as any)?.message || 'Unknown error'}</span>
        </div>
      ) : filteredImages.length === 0 ? (
        <div className="text-center py-16 border border-dashed border-slate-800 rounded-xl p-8">
          <Layers className="w-8 h-8 text-slate-600 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-slate-300">No images found</h3>
          <p className="text-xs text-slate-400 mt-1 max-w-sm mx-auto">
            {activeSearch
              ? 'No images match your filter criteria.'
              : 'There are no Docker images cached on the selected node(s).'}
          </p>
        </div>
      ) : (
        <div className="border border-slate-800/80 rounded-xl overflow-hidden bg-slate-900/20">
          <table className="w-full text-left text-xs border-collapse">
            <thead>
              <tr className="border-b border-slate-800 bg-slate-900/60 text-slate-400 font-medium">
                <th className="py-2.5 px-4">Repository & Tag</th>
                <th className="py-2.5 px-3">Image ID</th>
                <th className="py-2.5 px-3">Size</th>
                <th className="py-2.5 px-3">Containers</th>
                <th className="py-2.5 px-3">Created</th>
                <th className="py-2.5 px-3">Host Node</th>
                <th className="py-2.5 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 font-sans">
              {filteredImages.map((img) => {
                const tagList = img.RepoTags && img.RepoTags.length > 0 ? img.RepoTags : ['<none>:<none>'];
                const primaryTag = tagList[0];
                const extraTags = tagList.length - 1;
                const imgId = img.Id || img.ID || '';
                const shortId = imgId.replace('sha256:', '').substring(0, 12);
                const createdStr = img.Created ? new Date(img.Created * 1000).toLocaleDateString() : '-';

                return (
                  <tr
                    key={`${img.hostId}-${imgId}`}
                    onClick={() => setSelectedImage(img)}
                    className="hover:bg-slate-800/40 transition-colors cursor-pointer group"
                  >
                    <td className="py-3 px-4 min-w-0">
                      <div className="flex items-center gap-2">
                        <Layers className="w-4 h-4 text-slate-400 flex-shrink-0" />
                        <div className="min-w-0">
                          <div className="font-semibold text-slate-200 group-hover:text-white transition-colors truncate">
                            {primaryTag}
                          </div>
                          {extraTags > 0 && (
                            <span className="text-[10px] text-slate-400 font-mono">
                              +{extraTags} more tag{extraTags > 1 ? 's' : ''}
                            </span>
                          )}
                        </div>
                      </div>
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-400">
                      {shortId}
                    </td>
                    <td className="py-3 px-3 font-mono text-slate-300">
                      <div className="flex items-center gap-1.5">
                        <HardDrive className="w-3 h-3 text-slate-500" />
                        <span>{formatBytes(img.Size)}</span>
                      </div>
                    </td>
                    <td className="py-3 px-3">
                      <div className="flex items-center gap-1.5">
                        <Box className="w-3 h-3 text-slate-500" />
                        <span className="font-mono text-slate-300">{img.Containers}</span>
                      </div>
                    </td>
                    <td className="py-3 px-3 text-slate-400 font-mono text-[11px]">
                      {createdStr}
                    </td>
                    <td className="py-3 px-3 text-slate-400">
                      <div className="flex items-center gap-1.5">
                        <Server className="w-3 h-3 text-slate-400" />
                        <span className="truncate">{img.hostName}</span>
                      </div>
                    </td>
                    <td className="py-3 px-4 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedImage(img);
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

      {/* Image Detail Sheet */}
      <ImageDetailSheet
        image={selectedImage}
        isOpen={Boolean(selectedImage)}
        onClose={() => setSelectedImage(null)}
      />
    </div>
  );
};
