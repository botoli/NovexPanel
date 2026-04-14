import { makeAutoObservable } from 'mobx';
export const agentTokenStore = {
  agentToken: '',
  setAgentToken(newToken: string) {
    this.agentToken = newToken;
  },
  getAgentToken() {
    return this.agentToken;
  },
  clearAgentToken() {
    this.agentToken = '';
  },
};
makeAutoObservable(agentTokenStore);
