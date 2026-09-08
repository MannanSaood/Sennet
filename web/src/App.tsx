import { Suspense, lazy } from 'react';
import { Routes, Route } from 'react-router-dom';
import { ProtectedRoute } from '@/components/auth/ProtectedRoute';
import { ErrorBoundary } from '@/components/ui/ErrorBoundary';
import { PageLoader } from '@/components/ui/PageLoader';
import { ScrollToHashElement } from '@/components/ui/ScrollToHashElement';
const HomePage=lazy(()=>import('@/pages/home/HomePage').then(m=>({default:m.HomePage})));
const DocsPage=lazy(()=>import('@/pages/docs/DocsPage').then(m=>({default:m.DocsPage})));
const LoginPage=lazy(()=>import('@/pages/auth/LoginPage').then(m=>({default:m.LoginPage})));
const RegisterPage=lazy(()=>import('@/pages/auth/RegisterPage').then(m=>({default:m.RegisterPage})));
const Explorer=lazy(()=>import('@/features/observability/Explorer').then(m=>({default:m.Explorer})));
const SettingsPage=lazy(()=>import('@/pages/dashboard/SettingsPage').then(m=>({default:m.SettingsPage})));
const Monitors=lazy(()=>import('@/features/observability/Monitors').then(m=>({default:m.Monitors})));
const NotFoundPage=lazy(()=>import('@/pages/NotFoundPage').then(m=>({default:m.NotFoundPage})));
export default function App(){return <ErrorBoundary><Suspense fallback={<PageLoader/>}><ScrollToHashElement/><Routes><Route path="/" element={<HomePage/>}/><Route path="/docs/*" element={<DocsPage/>}/><Route path="/login" element={<LoginPage/>}/><Route path="/register" element={<RegisterPage/>}/><Route path="/dashboard" element={<ProtectedRoute><Explorer/></ProtectedRoute>}/>{[['traces','trace'],['logs','log'],['metrics','metric'],['traffic','flow'],['agents','agent'],['finance','finance']].map(([path,signal])=><Route key={path} path={'/dashboard/'+path} element={<ProtectedRoute><Explorer key={signal} signal={signal}/></ProtectedRoute>}/>)}<Route path="/dashboard/map" element={<ProtectedRoute><Explorer topology/></ProtectedRoute>}/><Route path="/dashboard/settings" element={<ProtectedRoute><SettingsPage/></ProtectedRoute>}/><Route path="/dashboard/alerts" element={<ProtectedRoute><Monitors/></ProtectedRoute>}/><Route path="*" element={<NotFoundPage/>}/></Routes></Suspense></ErrorBoundary>}
