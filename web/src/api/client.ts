import axios from 'axios';
import { getIdToken, isFirebaseConfigured } from '@/lib/firebase';

export const api = axios.create({ baseURL: import.meta.env.VITE_API_URL || '', timeout: 20000 });
api.interceptors.request.use(async config => {
    const token = isFirebaseConfigured ? await getIdToken() : null;
    const developmentToken = import.meta.env.DEV ? import.meta.env.VITE_SENNET_DEVELOPMENT_SESSION_TOKEN : undefined;
    if (token || developmentToken) config.headers.Authorization = `Bearer ${token || developmentToken}`;
    if (developmentToken && import.meta.env.VITE_SENNET_DEVELOPMENT_TENANT) config.headers['X-Sennet-Workspace'] = import.meta.env.VITE_SENNET_DEVELOPMENT_TENANT;
    return config;
});
api.interceptors.response.use(response => response, error => {
    if (error.response?.status === 401) window.dispatchEvent(new Event('sennet:unauthorized'));
    return Promise.reject(error);
});
