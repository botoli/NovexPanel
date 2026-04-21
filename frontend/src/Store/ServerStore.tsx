// hooks/useCurrentServer.ts
import { useParams } from 'react-router-dom';
import { serverMetricsStore } from './ServerMetricsStore';

export const useCurrentServer = () => {
  const { id } = useParams<{ id?: string; }>();
  const serverId = id ? Number(id) : NaN;
  const allServers = serverMetricsStore.getNowServers();
  const server = allServers.find(s => s.id === serverId);

  return { serverId, server, allServers };
};
