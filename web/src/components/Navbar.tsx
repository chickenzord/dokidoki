import React from 'react';
import { Link } from '@tanstack/react-router';
import { useNodesQuery } from '../hooks/useClusterData';
import { Button } from './ui/button';
import { NodeHealthDot } from './NodeHealthDot';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from './ui/dropdown-menu';
import {
  Activity,
  RefreshCw,
  Menu,
  X,
  Shield,
  ShieldCheck,
  KeyRound,
  LogOut,
} from 'lucide-react';
import { AuthStatus } from '../types';

interface NavbarProps {
  onRefresh: () => void;
  isRefreshing?: boolean;
  onToggleMobileSidebar?: () => void;
  isMobileSidebarOpen?: boolean;
  authStatus?: AuthStatus;
  onChangePasswordClick?: () => void;
  onLogoutClick?: () => void;
}

export const Navbar: React.FC<NavbarProps> = ({
  onRefresh,
  isRefreshing = false,
  onToggleMobileSidebar,
  isMobileSidebarOpen = false,
  authStatus,
  onChangePasswordClick,
  onLogoutClick,
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

        {/* Right Controls: Cluster status indicator + Refresh + Auth Menu */}
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

          {/* Auth Menu / Controls */}
          {authStatus && (
            authStatus.authenticated ? (
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button
                    size="sm"
                    variant="ghost"
                    className="h-8 px-2.5 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-300 hover:text-white flex items-center gap-1.5"
                    title="Account / Security"
                  >
                    <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                    <span className="text-xs font-medium hidden sm:inline">Admin</span>
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48 bg-slate-950 border-slate-800 text-slate-200">
                  <DropdownMenuLabel className="text-xs text-slate-400 font-normal">
                    Node Security
                  </DropdownMenuLabel>
                  <DropdownMenuSeparator className="bg-slate-800" />
                  <DropdownMenuItem
                    onClick={onChangePasswordClick}
                    className="cursor-pointer text-xs focus:bg-slate-900 focus:text-white"
                  >
                    <KeyRound className="w-3.5 h-3.5 mr-2 text-rose-400" />
                    Change Password
                  </DropdownMenuItem>
                  <DropdownMenuItem
                    onClick={onLogoutClick}
                    className="cursor-pointer text-xs text-rose-400 focus:bg-slate-900 focus:text-rose-300"
                  >
                    <LogOut className="w-3.5 h-3.5 mr-2" />
                    Log Out
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            ) : !authStatus.auth_required ? (
              <Button
                size="sm"
                variant="outline"
                onClick={onChangePasswordClick}
                className="h-8 px-2.5 rounded-lg border-amber-500/30 bg-amber-500/10 text-amber-300 hover:bg-amber-500/20 hover:text-amber-200 text-xs flex items-center gap-1.5"
                title="Standalone mode without password. Click to set admin password."
              >
                <Shield className="w-3.5 h-3.5 text-amber-400" />
                <span className="hidden sm:inline">Set Password</span>
              </Button>
            ) : null
          )}
        </div>
      </div>
    </header>
  );
};
