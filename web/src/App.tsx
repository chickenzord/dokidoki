import React, { useState, useEffect, useCallback } from 'react';
import { Node, HostInfo, StackSummary, ContainerSummary } from './types';
import { api } from './services/api';
import { Navbar } from './components/Navbar';
import { HostCard } from './components/HostCard';
import { StacksList } from './components/StacksList';
import { ContainersList } from './components/ContainersList';
import { Layers, Box, Activity } from 'lucide-react';

export const App: React.FC = () => {
  const [activeNode, setActiveNode] = useState<Node | null>(null);
  const [nodes, setNodes] = useState<Node[]>([]);
  const [hostInfo, setHostInfo] = useState<HostInfo | null>(null);
  const [stacks, setStacks] = useState<StackSummary[]>([]);
  const [containers, setContainers] = useState<ContainerSummary[]>([]);
  const [activeTab, setActiveTab] = useState<'stacks' | 'containers'>('stacks');
  const [selectedStackFilter, setSelectedStackFilter] = useState<string | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  // Fetch nodes list (always from cluster manager / local node)
  const fetchNodes = useCallback(async () => {
    try {
      const nodeList = await api.getNodes();
      setNodes(nodeList);
    } catch (err: any) {
      console.warn('Failed to fetch cluster nodes:', err.message);
    }
  }, []);

  // Fetch data for the active node
  const fetchData = useCallback(async () => {
    setLoading(true);
    setError(null);

    try {
      const [hostData, stacksData, containersData] = await Promise.all([
        api.getHostInfo().catch((err) => {
          console.warn('Host info fetch error:', err.message);
          return null;
        }),
        api.getStacks().catch((err) => {
          console.warn('Stacks fetch error:', err.message);
          return [];
        }),
        api.getContainers().catch((err) => {
          console.warn('Containers fetch error:', err.message);
          return [];
        }),
      ]);

      if (hostData) setHostInfo(hostData);
      setStacks(stacksData);
      setContainers(containersData);
    } catch (err: any) {
      setError(err.message || 'Failed to communicate with node');
    } finally {
      setLoading(false);
    }
  }, []);

  // Handle switching nodes
  const handleSelectNode = (node: Node | null) => {
    setActiveNode(node);
    if (node) {
      // Pick the first address (e.g. http://192.168.1.50:8080)
      const primaryAddr = node.addresses && node.addresses.length > 0 ? node.addresses[0] : '';
      api.setActiveEndpoint(primaryAddr);
    } else {
      api.setActiveEndpoint('');
    }
  };

  // Initial load and node switch trigger
  useEffect(() => {
    fetchNodes();
    fetchData();
  }, [fetchNodes, fetchData, activeNode]);

  // Navigate to container tab filtered by stack
  const handleSelectStack = (stackName: string) => {
    setSelectedStackFilter(stackName);
    setActiveTab('containers');
  };

  const clearStackFilter = () => {
    setSelectedStackFilter(null);
  };

  const totalRunningContainers = containers.filter(c => c.state.toLowerCase() === 'running').length;

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-rose-500/30 selection:text-rose-200">
      {/* Top Navbar */}
      <Navbar
        activeNode={activeNode}
        nodes={nodes}
        onSelectNode={handleSelectNode}
        onRefresh={() => {
          fetchNodes();
          fetchData();
        }}
        isLoading={loading}
      />

      {/* Main Content Area */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8 space-y-8">
        {/* Host Overview Card */}
        <HostCard hostInfo={hostInfo} loading={loading} error={error} />

        {/* Tab Navigation & Status Summary */}
        <div className="border-b border-slate-800">
          <div className="flex items-center justify-between flex-wrap gap-4 pb-1">
            <nav className="flex space-x-2" aria-label="Tabs">
              <button
                onClick={() => setActiveTab('stacks')}
                className={`flex items-center gap-2 py-3 px-4 rounded-xl text-sm font-semibold transition-all ${
                  activeTab === 'stacks'
                    ? 'bg-slate-900 text-rose-400 border border-slate-700/80 shadow-md shadow-black/20'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/50'
                }`}
              >
                <Layers className="w-4 h-4" />
                <span>Stacks</span>
                <span className="ml-1.5 px-2 py-0.5 text-xs rounded-full bg-slate-800 text-slate-300 font-mono">
                  {stacks.length}
                </span>
              </button>

              <button
                onClick={() => setActiveTab('containers')}
                className={`flex items-center gap-2 py-3 px-4 rounded-xl text-sm font-semibold transition-all ${
                  activeTab === 'containers'
                    ? 'bg-slate-900 text-rose-400 border border-slate-700/80 shadow-md shadow-black/20'
                    : 'text-slate-400 hover:text-slate-200 hover:bg-slate-900/50'
                }`}
              >
                <Box className="w-4 h-4" />
                <span>Containers</span>
                <span className="ml-1.5 px-2 py-0.5 text-xs rounded-full bg-slate-800 text-slate-300 font-mono">
                  {containers.length}
                </span>
                {totalRunningContainers > 0 && (
                  <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse ml-0.5" />
                )}
              </button>
            </nav>

            <div className="text-xs text-slate-500 flex items-center gap-2 font-mono">
              <Activity className="w-3.5 h-3.5 text-rose-500/80" />
              <span>
                {activeNode ? `Active Remote: ${activeNode.name}` : 'Local Node Active'}
              </span>
            </div>
          </div>
        </div>

        {/* Tab View */}
        <div>
          {activeTab === 'stacks' && (
            <StacksList
              stacks={stacks}
              containers={containers}
              loading={loading}
              error={error}
              onRefresh={fetchData}
              onSelectStack={handleSelectStack}
            />
          )}

          {activeTab === 'containers' && (
            <ContainersList
              containers={containers}
              loading={loading}
              error={error}
              onRefresh={fetchData}
              selectedStack={selectedStackFilter}
              onClearStackFilter={clearStackFilter}
            />
          )}
        </div>
      </main>
    </div>
  );
};
