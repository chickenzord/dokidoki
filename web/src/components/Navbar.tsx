import React from 'react';
import { Link } from '@tanstack/react-router';
import { useNodesQuery } from '../hooks/useClusterData';
import { Button } from './ui/button';
import { NodeHealthDot } from './NodeHealthDot';
import {
  Activity,
  RefreshCw,
  Menu,
  X,
} from 'lucide-react';

interface NavbarProps {
  onRefresh: () => void;
  isRefreshing?: boolean;
  onToggleMobileSidebar?: () => void;
  isMobileSidebarOpen?: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  onRefresh,
  isRefreshing = false,
  onToggleMobileSidebar,
  isMobileSidebarOpen = false,
}) => {
  const { data: nodes = [] } = useNodesQuery();

  const totalNodes = nodes.length;
  const aliveNodes = nodes.filter((n) => n.status === 'alive').length;
  const suspectNodes = nodes.filter((n) => n.status === 'suspect').length;
  const offlineNodes = nodes.filter((n) => n.status === 'offline').length;

  const clusterStatus =
    offlineNodes > 0
      ? 'degraded'
      : suspectNodes > 0
      ? 'warning'
      : totalNodes > 0
      ? 'healthy'
      : 'unknown';

  return (
    <header className="sticky top-0 z-40 w-full border-b border-slate-800/80 bg-slate-950/80 backdrop-blur-md">
      <div className="w-full px-4 sm:px-6 flex items-center justify-between h-14 gap-4">
        {/* Left: Mobile hamburger + Brand */}
        <div className="flex items-center gap-3">
          {onToggleMobileSidebar && (
            <Button
              size="sm"
              variant="ghost"
              onClick={onToggleMobileSidebar}
              className="md:hidden h-8 w-8 p-0 text-slate-400 hover:text-white hover:bg-slate-900 border border-slate-800"
              aria-label="Toggle navigation menu"
            >
              {isMobileSidebarOpen ? <X className="w-4 h-4" /> : <Menu className="w-4 h-4" />}
            </Button>
          )}

          <Link to="/stacks" className="flex items-center gap-2.5 group">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-tr from-rose-600 via-pink-500 to-rose-400 flex items-center justify-center shadow-md shadow-rose-950/50 group-hover:scale-105 transition-transform">
              <Activity className="w-4 h-4 text-white" />
            </div>
            <span className="font-bold text-base tracking-tight text-white group-hover:text-rose-400 transition-colors">
              Dokidoki
            </span>
          </Link>
        </div>

        {/* Right Controls: Cluster status indicator + Refresh */}
        <div className="flex items-center gap-3">
          {/* Cluster Status Indicator */}
          <div
            className="flex items-center gap-2 px-2.5 py-1 rounded-full bg-slate-900/90 border border-slate-800 text-xs font-mono"
            title={`${aliveNodes}/${totalNodes} nodes online${
              offlineNodes > 0 ? ` (${offlineNodes} offline)` : ''
            }`}
          >
            <NodeHealthDot status={clusterStatus} />
            <span className="text-slate-300 text-[11px] hidden sm:inline">
              Cluster:
            </span>
            <span className="text-slate-200 text-[11px]">
              {aliveNodes}/{totalNodes} node{totalNodes === 1 ? '' : 's'}
            </span>
          </div>

          {/* Global Refresh Button */}
          <Button
            size="sm"
            variant="ghost"
            onClick={onRefresh}
            disabled={isRefreshing}
            className="h-8 w-8 p-0 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-400 hover:text-white"
            title="Refresh cluster data"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isRefreshing ? 'animate-spin text-rose-500' : ''}`} />
          </Button>
        </div>
      </div>
    </header>
  );
};
