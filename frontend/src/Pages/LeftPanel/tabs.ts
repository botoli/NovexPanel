export interface Tab {
  id: number;
  name: string;
  path: string;
  svg: string;
}

export const Tabs: Tab[] = [
  {
    id: 0,
    name: "Overview",
    path: "/",
    svg: "/home-svgrepo-com.svg",
  },
  {
    id: 1,
    name: "Deploy",
    path: "/Deploy",
    svg: "/monitoring-health-svgrepo-com.svg",
  },
  {
    id: 2,
    name: "Containers",
    path: "/tasks",
    svg: "/tasks-list-svgrepo-com.svg",
  },
  {
    id: 3,
    name: "Terminal",
    path: "/Terminal",
    svg: "/code-svgrepo-com.svg",
  },
  {
    id: 4,
    name: "Logs",
    path: "/Logs",
    svg: "/settings-svgrepo-com.svg",
  },
  {
    id: 5,
    name: "System",
    path: "/System",
    svg: "/ai-ai-svgrepo-com.svg",
  },
];
