import axios from 'axios';

export const api = axios.create({
  baseURL: '/api'
});

api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

export const getLoginURL = () => api.get('/auth/github/login');
export const githubCallback = (code) => api.post('/auth/github/callback', { code });
export const bindIAMRole = (arn) => api.post('/auth/iam-role', { arn });
export const getRepos = () => api.get('/auth/github/repos');

export const getDeployments = () => api.get('/velzard/deployments');
export const triggerDeploy = (data) => api.post('/velzard/deploy', data);
export const destroyDeployment = (id) => api.post(`/velzard/destroy/${id}`);

export const getEnvironments = () => api.get('/zegion/environments');
export const terminateEnvironment = (id) => api.post(`/zegion/terminate/${id}`);
export const getTelemetrySummary = () => api.get('/telemetry/summary');

export const getAdminUsers = () => api.get('/admin/users');
export const adminDeleteUser = (id) => api.delete(`/admin/users/${id}`);
export const getAdminDeployments = () => api.get('/admin/deployments');
export const adminReconcile = () => api.post('/admin/system/reconcile');
export const getAdminSystemSummary = () => api.get('/admin/system/summary');
export const adminFlushSystem = () => api.delete('/admin/system/flush');
export const adminTerminateVelzard = (id) => api.post(`/admin/deployments/${id}/terminate`);
export const adminTerminateZegion = (id) => api.post(`/admin/zegion/terminate/${id}`);
