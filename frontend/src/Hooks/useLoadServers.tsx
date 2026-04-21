import { useCallback, useEffect, useRef } from 'react';
import { API_BASE } from '../Api/api';
import { serverMetricsStore } from '../Store/ServerMetricsStore';
import { tokenStore } from '../Store/TokenStore';

export const useLoadServers = () => {
  const isMounted = useRef(true);
  const isRequestInFlight = useRef(false);

  const loadServers = useCallback(async () => {
    if (isRequestInFlight.current) return;
    isRequestInFlight.current = true;

    try {
      const response = await fetch(`${API_BASE}/servers`, {
        headers: { Authorization: `Bearer ${tokenStore.getToken()}` },
      });
      if (!response.ok) throw new Error(`HTTP ${response.status}`);
      const data = await response.json();
      if (isMounted.current) {
        serverMetricsStore.setNowServers(data);

        serverMetricsStore.setServerMetricsError(null);
      }
    } catch (err) {
      serverMetricsStore.setServerMetricsError(
        err instanceof Error ? err.message : 'Failed to fetch servers',
      );
    } finally {
      serverMetricsStore.setServerMetricsLoading(false);
      isRequestInFlight.current = false;
    }
  }, [tokenStore]);

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
