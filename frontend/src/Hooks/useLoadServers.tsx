import { useCallback, useEffect, useRef } from 'react';
import { serverMetricsStore } from '../Store/ServerMetricsStore';
export const loading = false;
export const useLoadServers = () => {
  const isMounted = useRef(true);
  const isRequestInFlight = useRef(false);
  const didInitialLoad = useRef(false);

  const loadServers = useCallback(async () => {
    if (isRequestInFlight.current) return;
    isRequestInFlight.current = true;

    try {
      if (isMounted.current) {
        await serverMetricsStore.refreshNow({ silent: didInitialLoad.current });
        didInitialLoad.current = true;
      }
    } finally {
      isRequestInFlight.current = false;
    }
  }, []);

  useEffect(() => {
    loadServers();
    const interval = setInterval(loadServers, 2000);
    return () => {
      isMounted.current = false;
      clearInterval(interval);
    };
  }, [loadServers]);

  return { loadServers };
};
