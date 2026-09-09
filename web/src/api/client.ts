import axios from 'axios';
import { getIdToken, isFirebaseConfigured } from '@/lib/firebase';

export const api = axios.create({ baseURL: import.meta.env.VITE_API_URL || '', timeout: 20000 });
api.interceptors.request.use(async config => {
    const token = isFirebaseConfigured ? await getIdToken() : null;
    if (token) config.headers.Authorization = `Bearer ${token}`;
    return config;
});
api.interceptors.response.use(response => response, error => {
    if (error.response?.status === 401) window.dispatchEvent(new Event('sennet:unauthorized'));
    return Promise.reject(error);
});
