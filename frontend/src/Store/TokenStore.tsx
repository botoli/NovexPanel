import { makeAutoObservable } from 'mobx';
export const tokenStore = {
  token: '',
  setToken(newToken: string) {
    this.token = newToken;
    localStorage.setItem('token', newToken);
  },
  getToken() {
    if (!this.token) {
      this.token = localStorage.getItem('token') || '';
    }
    return this.token;
  },
  clearToken() {
    this.token = '';
    localStorage.removeItem('token');
  },
};
makeAutoObservable(tokenStore);
