import { useEffect, useState } from 'react';
import type { Release } from './types';

export function useReleases() {
  const [releases, setReleases] = useState<Release[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    let controller: AbortController | undefined;
    let requestID = 0;
    let disposed = false;
    async function refresh() {
      controller?.abort();
      controller = new AbortController();
      const id = ++requestID;
      try {
        const response = await fetch('/api/releases', {
          credentials: 'same-origin', signal: controller.signal,
        });
        if (!response.ok) throw new Error('Unable to load releases');
        const data: unknown = await response.json();
        // The Go store can encode its empty list as either [] or null.
        if (data !== null && !Array.isArray(data)) throw new Error('Invalid release list');
        if (disposed || id !== requestID) return;
        setReleases((data ?? []) as Release[]);
        setError(false);
      } catch {
        if (!disposed && id === requestID) setError(true);
      } finally {
        if (!disposed && id === requestID) setLoading(false);
      }
    }
    void refresh();
    window.addEventListener('focus', refresh);
    return () => {
      disposed = true;
      controller?.abort();
      window.removeEventListener('focus', refresh);
    };
  }, []);
  return { releases, loading, error };
}

export function useMotion() {
  const [paused, setPaused] = useState(() => {
    try { return localStorage.getItem('colday-motion-paused') === '1'; }
    catch { return false; }
  });
  const [reduced, setReduced] = useState(() => matchMedia('(prefers-reduced-motion: reduce)').matches);
  const [hidden, setHidden] = useState(document.hidden);

  useEffect(() => {
    const query = matchMedia('(prefers-reduced-motion: reduce)');
    const onPreference = () => setReduced(query.matches);
    const onVisibility = () => setHidden(document.hidden);
    query.addEventListener('change', onPreference);
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      query.removeEventListener('change', onPreference);
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, []);

  useEffect(() => {
    try { localStorage.setItem('colday-motion-paused', paused ? '1' : '0'); } catch { /* Optional storage. */ }
  }, [paused]);

  return { paused, reduced, disabled: paused || reduced || hidden, toggle: () => setPaused(value => !value) };
}
