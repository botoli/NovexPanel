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
    id: 3,
    name: 'Network',
    path: '/Network',
  },
  {
    id: 4,
    name: 'Domains',
    path: '/Domains',
  },
  {
    id: 5,
    name: 'AI Guard',
    path: '/AIGuard',
  },
  {
    id: 6,
    name: 'Auto Heal',
    path: '/AutoHeal',
  },
  {
    id: 7,
    name: 'Settings',
    path: '/Settings',
  },
];
