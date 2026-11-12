import React from 'react';
import { Link } from '@tanstack/react-router';
import { Node } from '../types';
import { Badge } from './ui/badge';
import { Button } from './ui/button';
import { Server, ArrowLeft, Layers, Box, Cpu } from 'lucide-react';
import { useSmartBack } from '../lib/navigation';

export interface NodeContextHeaderProps {
  nodeId: string;
  node?: Node;
  fallbackTo?: string;
  activeSection?: 'containers' | 'stacks' | 'overview';
}

export const NodeContextHeader: React.FC<NodeContextHeaderProps> = ({
  nodeId,
  node,
  fallbackTo = '/nodes',
  activeSection,
}) => {
  const { goBackOr } = useSmartBack();

  const nodeName = node?.name || (nodeId === 'local' ? 'Local' : nodeId.substring(0, 8));
  const isAlive = node?.status === 'alive';
  const isSuspect = node?.status === 'suspect';
  const isOffline = node?.status === 'offline';
  const isReadOnly = Boolean(node?.read_only);
  const primaryAddress = node?.addresses?.[0];

  const handleBack = () => {
    goBackOr(fallbackTo);
  };

  return (
    <div className="rounded-xl border border-slate-800 bg-slate-900/50 p-4 shadow-sm backdrop-blur-sm space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        {/* Left: Back button + Node info */}
        <div className="flex items-center gap-3 min-w-0">
          <Button
            size="sm"
            variant="outline"
            onClick={handleBack}
            className="h-8 px-2.5 text-xs border-slate-800 bg-slate-900 hover:bg-slate-800 text-slate-300 hover:text-white shrink-0"
            title="Go back"
          >
            <ArrowLeft className="w-3.5 h-3.5 mr-1" />
            Back
          </Button>

          <div className="flex items-center gap-2.5 min-w-0">
            <Link
              to="/nodes/$nodeId"
              params={{ nodeId }}
              className="group flex items-center gap-2 min-w-0 hover:opacity-90 transition-opacity"
            >
              <div className="w-7 h-7 rounded-lg bg-emerald-950/50 border border-emerald-800/60 flex items-center justify-center shrink-0 text-emerald-400 group-hover:border-emerald-700">
                <Server className="w-4 h-4" />
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <span className="font-semibold text-sm text-white group-hover:text-rose-400 transition-colors truncate">
                    {nodeName}
                  </span>
                  <span className="text-[10px] text-slate-500 font-mono hidden sm:inline">
                    (Node View)
                  </span>
                </div>
                <div className="text-[11px] text-slate-400 font-mono truncate flex items-center gap-2">
                  <span>{primaryAddress || (nodeId === 'local' ? 'Direct socket' : 'No address')}</span>
                  {node?.version && (
                    <>
                      <span className="text-slate-600">•</span>
                      <span>v{node.version}</span>
                    </>
                  )}
                </div>
              </div>
            </Link>
          </div>
        </div>

        {/* Right: Badges & Quick Nav */}
        <div className="flex flex-wrap items-center gap-2 shrink-0">
          {node?.status && (
            <Badge
              variant="outline"
              className={`text-[10px] uppercase font-mono ${
                isAlive
                  ? 'bg-emerald-950/40 text-emerald-400 border-emerald-800/60'
                  : isSuspect
                  ? 'bg-amber-950/40 text-amber-400 border-amber-800/60'
                  : isOffline
                  ? 'bg-rose-950/40 text-rose-400 border-rose-800/60'
                  : 'bg-slate-800 text-slate-400 border-slate-700'
              }`}
            >
              {node.status}
            </Badge>
          )}

          {node?.is_self && (
            <Badge variant="outline" className="text-[10px] bg-slate-800 text-slate-300 border-slate-700">
              Current Origin
            </Badge>
          )}

          {isReadOnly ? (
            <Badge variant="outline" className="text-[10px] bg-amber-950/40 text-amber-400 border-amber-800/60 font-mono">
              Read-Only
            </Badge>
          ) : (
            <Badge variant="outline" className="text-[10px] bg-slate-900 text-slate-400 border-slate-800 font-mono">
              Read/Write
            </Badge>
          )}

          {/* Quick links to node's other tabs */}
          <div className="h-4 w-px bg-slate-800 mx-1 hidden sm:block" />

          <Link to="/nodes/$nodeId" params={{ nodeId }}>
            <Button
              size="sm"
              variant="ghost"
              className={`h-7 px-2 text-xs ${activeSection === 'overview' ? 'text-white bg-slate-800' : 'text-slate-400 hover:text-white'}`}
            >
              <Cpu className="w-3 h-3 mr-1" />
              Node
            </Button>
          </Link>

          <Link to="/nodes/$nodeId/stacks" params={{ nodeId }}>
            <Button
              size="sm"
              variant="ghost"
              className={`h-7 px-2 text-xs ${activeSection === 'stacks' ? 'text-rose-400 bg-rose-950/30' : 'text-slate-400 hover:text-white'}`}
            >
              <Layers className="w-3 h-3 mr-1" />
              Stacks
            </Button>
          </Link>

          <Link to="/nodes/$nodeId/containers" params={{ nodeId }}>
            <Button
              size="sm"
              variant="ghost"
              className={`h-7 px-2 text-xs ${activeSection === 'containers' ? 'text-blue-400 bg-blue-950/30' : 'text-slate-400 hover:text-white'}`}
            >
              <Box className="w-3 h-3 mr-1" />
              Containers
            </Button>
          </Link>
        </div>
      </div>
    </div>
  );
};
