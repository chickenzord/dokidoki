import { useState, useEffect, createContext, useContext } from 'react';
import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
  useParams,
} from '@tanstack/react-router';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { Navbar } from './components/Navbar';
import { TreeSidebar } from './components/TreeSidebar';
import { Sheet, SheetContent, SheetTitle } from './components/ui/sheet';
import { ClusterStacksView } from './components/ClusterStacksView';
import { ClusterContainersView } from './components/ClusterContainersView';
import { ClusterNodesView } from './components/ClusterNodesView';
import { ClusterNetworksView } from './components/ClusterNetworksView';
import { ClusterVolumesView } from './components/ClusterVolumesView';
import { ClusterImagesView } from './components/ClusterImagesView';
import { EventsView } from './components/EventsView';
import { NodeDetailView } from './pages/NodeDetailView';
import { NodeStackDetailRouteComponent } from './pages/NodeStackDetailRouteComponent';
import { NodeContainerDetailRouteComponent } from './pages/NodeContainerDetailRouteComponent';
import { useDocumentTitle } from './hooks/useDocumentTitle';
import { useNodesQuery } from './hooks/useClusterData';
import { BreadcrumbNav } from './components/BreadcrumbNav';
import { AuthLockScreen } from './components/AuthLockScreen';
import { ChangePasswordDialog } from './components/ChangePasswordDialog';
import { api } from './services/api';
import { Server, Layers, Box, Network, HardDrive } from 'lucide-react';

export interface ClusterUIContextType {
  selectedHostId: string | null;
  setSelectedHostId: (host: string | null) => void;
  searchQuery: string;
  setSearchQuery: (query: string) => void;
}

export const ClusterUIContext = createContext<ClusterUIContextType>({
  selectedHostId: null,
  setSelectedHostId: () => {},
  searchQuery: '',
  setSearchQuery: () => {},
});

export const useClusterUI = () => useContext(ClusterUIContext);

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 5000,
      retry: 1,
    },
  },
});

function InnerLayout() {
  const [selectedHostId, setSelectedHostId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [changePasswordOpen, setChangePasswordOpen] = useState(false);
  const [isAuthLocked, setIsAuthLocked] = useState(false);

  const { data: authStatus, refetch: refetchAuth } = useQuery({
    queryKey: ['authStatus'],
    queryFn: () => api.getAuthStatus(),
    refetchInterval: 15000,
  });

  // Global 401 handler
  useEffect(() => {
    const unsubscribe = api.onUnauthorized(() => {
      setIsAuthLocked(true);
      refetchAuth();
    });
    return unsubscribe;
  }, [refetchAuth]);

  // If backend requires auth and user is not authenticated, lock screen
  const shouldShowLockScreen = isAuthLocked || (authStatus?.auth_required && !authStatus?.authenticated);

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await Promise.all([queryClient.invalidateQueries(), refetchAuth()]);
    setTimeout(() => setIsRefreshing(false), 400);
  };

  const handleLoginSuccess = async () => {
    setIsAuthLocked(false);
    await refetchAuth();
    await queryClient.invalidateQueries();
  };

  const handleLogout = async () => {
    try {
      await api.logout();
    } finally {
      await refetchAuth();
      await queryClient.invalidateQueries();
      setIsAuthLocked(true);
    }
  };

  return (
    <ClusterUIContext.Provider
      value={{
        selectedHostId,
        setSelectedHostId,
        searchQuery,
        setSearchQuery,
      }}
    >
      <div className="min-h-screen bg-slate-950 text-slate-100 flex flex-col font-sans selection:bg-rose-500/20 selection:text-rose-300">
        <Navbar
          onRefresh={handleRefresh}
          isRefreshing={isRefreshing}
          onToggleMobileSidebar={() => setMobileSidebarOpen(!mobileSidebarOpen)}
          isMobileSidebarOpen={mobileSidebarOpen}
          authStatus={authStatus}
          onChangePasswordClick={() => setChangePasswordOpen(true)}
          onLogoutClick={handleLogout}
        />

        {/* Main Layout: Sidebar + Route Workspace */}
        <div className="flex-1 flex overflow-hidden">
          {/* Desktop Tree Sidebar */}
          <div className="hidden md:block flex-shrink-0">
            <TreeSidebar />
          </div>

          {/* Mobile Sheet Tree Sidebar */}
          <Sheet open={mobileSidebarOpen} onOpenChange={setMobileSidebarOpen}>
            <SheetContent side="left" className="p-0 w-72 bg-slate-950 border-r border-slate-800">
              <SheetTitle className="sr-only">Navigation</SheetTitle>
              <TreeSidebar onItemClick={() => setMobileSidebarOpen(false)} />
            </SheetContent>
          </Sheet>

          {/* Main Content Workspace */}
          <main className="flex-1 overflow-y-auto px-4 sm:px-6 py-6 min-w-0">
            <div className="max-w-7xl mx-auto">
              <Outlet />
            </div>
          </main>
        </div>

        {/* Auth Lock Screen Modal */}
        <AuthLockScreen
          open={!!shouldShowLockScreen}
          onSuccess={handleLoginSuccess}
        />

        {/* Change Password Dialog */}
        <ChangePasswordDialog
          open={changePasswordOpen}
          onOpenChange={setChangePasswordOpen}
          requireCurrentPassword={!!authStatus?.auth_required}
          onSuccess={async () => {
            await refetchAuth();
          }}
        />
      </div>
    </ClusterUIContext.Provider>
  );
}

function RootComponent() {
  return (
    <QueryClientProvider client={queryClient}>
      <InnerLayout />
    </QueryClientProvider>
  );
}

const rootRoute = createRootRoute({
  component: RootComponent,
});

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  beforeLoad: () => {
    throw redirect({ to: '/stacks' });
  },
});

interface StacksSearch {
  host?: string;
  selected?: string;
}

const stacksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/stacks',
  validateSearch: (search: Record<string, unknown>): StacksSearch => ({
    host: typeof search.host === 'string' ? search.host : undefined,
    selected: typeof search.selected === 'string' ? search.selected : undefined,
  }),
  component: StacksRouteComponent,
});

function StacksRouteComponent() {
  useDocumentTitle('Stacks');
  const search = stacksRoute.useSearch();
  const { selectedHostId, searchQuery } = useClusterUI();
  const effectiveHost =
    search.host !== undefined
      ? search.host === 'all'
        ? null
        : search.host
      : selectedHostId;

  return (
    <ClusterStacksView
      selectedHostId={effectiveHost}
      searchQuery={searchQuery}
    />
  );
}

interface ContainersSearch {
  host?: string;
  selected?: string;
}

const containersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/containers',
  validateSearch: (search: Record<string, unknown>): ContainersSearch => ({
    host: typeof search.host === 'string' ? search.host : undefined,
    selected: typeof search.selected === 'string' ? search.selected : undefined,
  }),
  component: ContainersRouteComponent,
});

function ContainersRouteComponent() {
  useDocumentTitle('Containers');
  const search = containersRoute.useSearch();
  const { selectedHostId, searchQuery } = useClusterUI();
  const effectiveHost =
    search.host !== undefined
      ? search.host === 'all'
        ? null
        : search.host
      : selectedHostId;

  return (
    <ClusterContainersView
      selectedHostId={effectiveHost}
      searchQuery={searchQuery}
    />
  );
}

const nodesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes',
  component: NodesRouteComponent,
});

function NodesRouteComponent() {
  useDocumentTitle('Nodes');
  return <ClusterNodesView />;
}

// Global Networks Route: /networks
interface NetworksSearch {
  host?: string;
}

const networksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/networks',
  validateSearch: (search: Record<string, unknown>): NetworksSearch => ({
    host: typeof search.host === 'string' ? search.host : undefined,
  }),
  component: NetworksRouteComponent,
});

function NetworksRouteComponent() {
  useDocumentTitle('Networks');
  const search = networksRoute.useSearch();
  const { selectedHostId, searchQuery } = useClusterUI();
  const effectiveHost =
    search.host !== undefined
      ? search.host === 'all'
        ? null
        : search.host
      : selectedHostId;

  return (
    <ClusterNetworksView
      selectedHostId={effectiveHost}
      searchQuery={searchQuery}
    />
  );
}

// Global Volumes Route: /volumes
interface VolumesSearch {
  host?: string;
}

const volumesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/volumes',
  validateSearch: (search: Record<string, unknown>): VolumesSearch => ({
    host: typeof search.host === 'string' ? search.host : undefined,
  }),
  component: VolumesRouteComponent,
});

function VolumesRouteComponent() {
  useDocumentTitle('Volumes');
  const search = volumesRoute.useSearch();
  const { selectedHostId, searchQuery } = useClusterUI();
  const effectiveHost =
    search.host !== undefined
      ? search.host === 'all'
        ? null
        : search.host
      : selectedHostId;

  return (
    <ClusterVolumesView
      selectedHostId={effectiveHost}
      searchQuery={searchQuery}
    />
  );
}

// Global Images Route: /images
interface ImagesSearch {
  host?: string;
}

const imagesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/images',
  validateSearch: (search: Record<string, unknown>): ImagesSearch => ({
    host: typeof search.host === 'string' ? search.host : undefined,
  }),
  component: ImagesRouteComponent,
});

function ImagesRouteComponent() {
  useDocumentTitle('Images');
  const search = imagesRoute.useSearch();
  const { selectedHostId, searchQuery } = useClusterUI();
  const effectiveHost =
    search.host !== undefined
      ? search.host === 'all'
        ? null
        : search.host
      : selectedHostId;

  return (
    <ClusterImagesView
      selectedHostId={effectiveHost}
      searchQuery={searchQuery}
    />
  );
}

// Global Events Route: /events
interface EventsSearch {
  host?: string;
}

const eventsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/events',
  validateSearch: (search: Record<string, unknown>): EventsSearch => ({
    host: typeof search.host === 'string' ? search.host : undefined,
  }),
  component: EventsRouteComponent,
});

function EventsRouteComponent() {
  useDocumentTitle('Events');
  const search = eventsRoute.useSearch();
  const { selectedHostId } = useClusterUI();
  const effectiveHost =
    search.host !== undefined
      ? search.host === 'all'
        ? null
        : search.host
      : selectedHostId;

  return <EventsView selectedHostId={effectiveHost} />;
}

// Node-scoped route 1: /nodes/$nodeId
const nodeDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId',
  component: NodeDetailRouteComponent,
});

function NodeDetailRouteComponent() {
  const { nodeId } = useParams({ from: '/nodes/$nodeId' });
  return <NodeDetailView nodeId={nodeId} />;
}

// Node-scoped route 2: /nodes/$nodeId/stacks
const nodeStacksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId/stacks',
  component: NodeStacksRouteComponent,
});

function NodeStacksRouteComponent() {
  const { nodeId } = useParams({ from: '/nodes/$nodeId/stacks' });
  const { searchQuery } = useClusterUI();
  const { data: nodes = [] } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const nodeLabel = node?.name || (nodeId === 'local' ? 'Local' : (nodeId ? nodeId.substring(0, 8) : ''));
  useDocumentTitle(`Stacks on ${nodeLabel}`);

  return (
    <div className="space-y-4">
      {/* Top Breadcrumbs */}
      <BreadcrumbNav
        items={[
          {
            label: nodeLabel,
            to: '/nodes/$nodeId',
            params: { nodeId },
            icon: Server,
          },
          {
            label: 'Stacks',
            icon: Layers,
          },
        ]}
      />

      <ClusterStacksView selectedHostId={nodeId} searchQuery={searchQuery} />
    </div>
  );
}

// Node-scoped route 3: /nodes/$nodeId/stacks/$stackName
const nodeStackDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId/stacks/$stackName',
  component: NodeStackDetailRouteComponent,
});

// Node-scoped route 4: /nodes/$nodeId/containers
const nodeContainersRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId/containers',
  component: NodeContainersRouteComponent,
});

function NodeContainersRouteComponent() {
  const { nodeId } = useParams({ from: '/nodes/$nodeId/containers' });
  const { searchQuery } = useClusterUI();
  const { data: nodes = [] } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const nodeLabel = node?.name || (nodeId === 'local' ? 'Local' : (nodeId ? nodeId.substring(0, 8) : ''));
  useDocumentTitle(`Containers on ${nodeLabel}`);

  return (
    <div className="space-y-4">
      {/* Top Breadcrumbs */}
      <BreadcrumbNav
        items={[
          {
            label: nodeLabel,
            to: '/nodes/$nodeId',
            params: { nodeId },
            icon: Server,
          },
          {
            label: 'Containers',
            icon: Box,
          },
        ]}
      />

      <ClusterContainersView selectedHostId={nodeId} searchQuery={searchQuery} />
    </div>
  );
}

// Node-scoped route 5: /nodes/$nodeId/containers/$containerId
const nodeContainerDetailRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId/containers/$containerId',
  component: NodeContainerDetailRouteComponent,
});

// Node-scoped route 6: /nodes/$nodeId/networks
const nodeNetworksRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId/networks',
  component: NodeNetworksRouteComponent,
});

function NodeNetworksRouteComponent() {
  const { nodeId } = useParams({ from: '/nodes/$nodeId/networks' });
  const { searchQuery } = useClusterUI();
  const { data: nodes = [] } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const nodeLabel = node?.name || (nodeId === 'local' ? 'Local' : (nodeId ? nodeId.substring(0, 8) : ''));
  useDocumentTitle(`Networks on ${nodeLabel}`);

  return (
    <div className="space-y-4">
      <BreadcrumbNav
        items={[
          {
            label: nodeLabel,
            to: '/nodes/$nodeId',
            params: { nodeId },
            icon: Server,
          },
          {
            label: 'Networks',
            icon: Network,
          },
        ]}
      />

      <ClusterNetworksView selectedHostId={nodeId} searchQuery={searchQuery} />
    </div>
  );
}

// Node-scoped route 7: /nodes/$nodeId/volumes
const nodeVolumesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId/volumes',
  component: NodeVolumesRouteComponent,
});

function NodeVolumesRouteComponent() {
  const { nodeId } = useParams({ from: '/nodes/$nodeId/volumes' });
  const { searchQuery } = useClusterUI();
  const { data: nodes = [] } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const nodeLabel = node?.name || (nodeId === 'local' ? 'Local' : (nodeId ? nodeId.substring(0, 8) : ''));
  useDocumentTitle(`Volumes on ${nodeLabel}`);

  return (
    <div className="space-y-4">
      <BreadcrumbNav
        items={[
          {
            label: nodeLabel,
            to: '/nodes/$nodeId',
            params: { nodeId },
            icon: Server,
          },
          {
            label: 'Volumes',
            icon: HardDrive,
          },
        ]}
      />

      <ClusterVolumesView selectedHostId={nodeId} searchQuery={searchQuery} />
    </div>
  );
}

// Node-scoped route 8: /nodes/$nodeId/images
const nodeImagesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/nodes/$nodeId/images',
  component: NodeImagesRouteComponent,
});

function NodeImagesRouteComponent() {
  const { nodeId } = useParams({ from: '/nodes/$nodeId/images' });
  const { searchQuery } = useClusterUI();
  const { data: nodes = [] } = useNodesQuery();
  const node = nodes.find((n) => n.id === nodeId);
  const nodeLabel = node?.name || (nodeId === 'local' ? 'Local' : (nodeId ? nodeId.substring(0, 8) : ''));
  useDocumentTitle(`Images on ${nodeLabel}`);

  return (
    <div className="space-y-4">
      <BreadcrumbNav
        items={[
          {
            label: nodeLabel,
            to: '/nodes/$nodeId',
            params: { nodeId },
            icon: Server,
          },
          {
            label: 'Images',
            icon: Layers,
          },
        ]}
      />

      <ClusterImagesView selectedHostId={nodeId} searchQuery={searchQuery} />
    </div>
  );
}

const routeTree = rootRoute.addChildren([
  indexRoute,
  stacksRoute,
  containersRoute,
  nodesRoute,
  networksRoute,
  volumesRoute,
  imagesRoute,
  eventsRoute,
  nodeDetailRoute,
  nodeStacksRoute,
  nodeStackDetailRoute,
  nodeContainersRoute,
  nodeContainerDetailRoute,
  nodeNetworksRoute,
  nodeVolumesRoute,
  nodeImagesRoute,
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
