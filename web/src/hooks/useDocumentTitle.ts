import { useEffect } from 'react';

/**
 * Hook to dynamically set document.title.
 * Formats title as: "<title> | Dokidoki" or defaults to "Dokidoki" if empty.
 */
export function useDocumentTitle(title?: string | null) {
  useEffect(() => {
    if (title && title.trim().length > 0) {
      document.title = `${title.trim()} | Dokidoki`;
    } else {
      document.title = 'Dokidoki';
    }
  }, [title]);
}
