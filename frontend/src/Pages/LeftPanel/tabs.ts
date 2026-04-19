export interface Tab {
  id: number;
  name: string;
  path: string;
}

export const Tabs: Tab[] = [
  {
    id: 0,
    name: 'Servers',
    path: '/',
  },
  {
    id: 1,
    name: 'Deploy',
    path: '/Deploy',
  },
  {
    id: 2,
    name: 'Containers',
    path: '/Containers',
  },
  {
    id: 3,
    name: 'Terminal',
    path: '/Terminal',
  },
  {
    id: 4,
    name: 'Logs',
    path: '/Logs',
  },
  {
    id: 5,
    name: 'System',
    path: '/System',
  },
];
