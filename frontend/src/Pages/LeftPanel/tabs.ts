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
    name: 'Applications',
    path: '/Applications',
  },
  {
    id: 2,
    name: 'Activity',
    path: '/Activity',
  },
  {
    id: 5,
    name: 'Settings',
    path: '/Settings',
  },
];
