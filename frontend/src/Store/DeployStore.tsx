import { makeAutoObservable } from 'mobx';

export const DeployStore = {
  deployId: null as number | null,
  setDeployId(id: number) {
    this.deployId = id;
    localStorage.setItem('deployId', id.toString());
  },
  getDeployId() {
    return Number(localStorage.getItem('deployId'));
  },
};
makeAutoObservable(DeployStore);
