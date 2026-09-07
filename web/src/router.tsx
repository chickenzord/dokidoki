import { useState, createContext, useContext } from 'react';
import {
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  redirect,
} from '@tanstack/react-router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { Navbar } from './components/Navbar';
import { ClusterStacksView } from './components/ClusterStacksView';
import { ClusterContainersView } from './components/ClusterContainersView';
import { ClusterNodesView } from './components/ClusterNodesView';

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

function RootComponent() {
  const [selectedHostId, setSelectedHostId] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await queryClient.invalidateQueries();
    setTimeout(() => setIsRefreshing(false), 400);
  };

  return (
    <QueryClientProvider client={queryClient}>
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
            selectedHostId={selectedHostId}
            onSelectHostId={setSelectedHostId}
            searchQuery={searchQuery}
            onSearchChange={setSearchQuery}
            onRefresh={handleRefresh}
            isRefreshing={isRefreshing}
          />
          <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-6">
            <Outlet />
          </main>
        </div>
      </ClusterUIContext.Provider>
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
  component: ClusterNodesView,
});

const routeTree = rootRoute.addChildren([
  indexRoute,
  stacksRoute,
  containersRoute,
  nodesRoute,
]);

export const router = createRouter({ routeTree });

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router;
  }
}
