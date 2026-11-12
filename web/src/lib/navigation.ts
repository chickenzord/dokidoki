import { useCallback } from 'react';
import { useRouter } from '@tanstack/react-router';

/**
 * Returns whether the user has prior history entries within this app session.
 * Checks both browser history length and TanStack Router's internal history index state.
 */
export function canGoBackInHistory(router?: ReturnType<typeof useRouter>): boolean {
  if (typeof window === 'undefined') return false;

  // 1. If router is available, check TanStack Router's internal history index (__TSR_index).
  // A fresh load or direct entry begins at index 0.
  if (router?.history?.location?.state?.__TSR_index !== undefined) {
    return router.history.location.state.__TSR_index > 0;
  }

  // Also check window.history.state if available
  const state = window.history.state;
  if (state && typeof state.__TSR_index === 'number') {
    return state.__TSR_index > 0;
  }

  // 2. Fallback to standard window.history.length
  return window.history.length > 1;
}

/**
 * Navigate back if there is preceding history in the app; otherwise replace state with fallback path.
 */
export function navigateBackOr(
  router: ReturnType<typeof useRouter>,
  fallback: { to: string; params?: Record<string, any>; search?: Record<string, any> } | string
) {
  if (canGoBackInHistory(router)) {
    window.history.back();
  } else {
    if (typeof fallback === 'string') {
      router.navigate({ to: fallback, replace: true });
    } else {
      router.navigate({ ...fallback, replace: true });
    }
  }
}

/**
 * Hook providing a smart back navigation function.
 */
export function useSmartBack() {
  const router = useRouter();

  const goBackOr = useCallback(
    (fallback: { to: string; params?: Record<string, any>; search?: Record<string, any> } | string) => {
      navigateBackOr(router, fallback);
    },
    [router]
  );

  return { goBackOr, canGoBack: canGoBackInHistory(router) };
}
