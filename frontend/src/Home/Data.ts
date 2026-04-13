export interface ServerState {
  name: string;
  status: "online" | "offline";
}

export interface ResourceMetric {
  id: number;
  label: string;
  value: number;
  used: number;
  total: number;
  unit: string;
  icon: "cpu" | "memory" | "disk";
}

export interface HighlightCard {
  id: number;
  label: string;
  value: string;
  note: string;
  icon: "containers" | "deploy";
}

export interface ActivityItem {
  id: number;
  type: "success" | "warning" | "info" | "error";
  message: string;
  target: string;
  ago: string;
}

export interface DashboardMock {
  title: string;
  server: ServerState;
  metrics: ResourceMetric[];
  highlights: HighlightCard[];
  activity: ActivityItem[];
}

export const dashboardMock: DashboardMock = {
  title: "Overview",
  server: {
    name: "myserver",
    status: "online",
  },
  metrics: [
    {
      id: 1,
      label: "CPU",
      value: 38,
      used: 38,
      total: 100,
      unit: "%",
      icon: "cpu",
    },
    {
      id: 2,
      label: "Memory",
      value: 52,
      used: 4.2,
      total: 8,
      unit: "GB",
      icon: "memory",
    },
    {
      id: 3,
      label: "Disks",
      value: 24,
      used: 32,
      total: 128,
      unit: "GB",
      icon: "disk",
    },
  ],
  highlights: [
    {
      id: 1,
      label: "Containers running",
      value: "4",
      note: "active now",
      icon: "containers",
    },
    {
      id: 2,
      label: "Deploy status",
      value: "1 queued",
      note: "waiting pipeline",
      icon: "deploy",
    },
  ],
  activity: [
    {
      id: 1,
      type: "success",
      message: "Successfully deployed",
      target: "project-frontend",
      ago: "5m ago",
    },
    {
      id: 2,
      type: "info",
      message: "Nginx container",
      target: "restarted",
      ago: "12m ago",
    },
    {
      id: 3,
      type: "info",
      message: "New SSH session",
      target: "opened",
      ago: "18m ago",
    },
    {
      id: 4,
      type: "error",
      message: "Backup job failed",
      target: "backup-job",
      ago: "24m ago",
    },
  ],
};
