import { Suspense } from 'react'
import { Routes, Route } from 'react-router-dom'
import { HomePage } from '@/pages/home/HomePage'
import { DocsPage } from '@/pages/docs/DocsPage'
import { LoginPage } from '@/pages/auth/LoginPage'
import { RegisterPage } from '@/pages/auth/RegisterPage'
import { ProtectedRoute } from '@/components/auth/ProtectedRoute'
import { DashboardPage } from '@/pages/dashboard/DashboardPage'
import { LiveTrafficPage } from '@/pages/dashboard/LiveTrafficPage'
import { ServiceMapPage } from '@/pages/dashboard/ServiceMapPage'
import { SettingsPage } from '@/pages/dashboard/SettingsPage'
import { NotFoundPage } from '@/pages/NotFoundPage'
import { ErrorBoundary } from '@/components/ui/ErrorBoundary'
import { PageLoader } from '@/components/ui/PageLoader'
import { ScrollToHashElement } from '@/components/ui/ScrollToHashElement'

function App() {
  return (
    <ErrorBoundary>
      <Suspense fallback={<PageLoader />}>
        <ScrollToHashElement />
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/docs" element={<DocsPage />} />
          <Route path="/docs/*" element={<DocsPage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />

          {/* Dashboard Routes */}
          <Route
            path="/dashboard"
            element={
              <ProtectedRoute>
                <DashboardPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard/traffic"
            element={
              <ProtectedRoute>
                <LiveTrafficPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard/map"
            element={
              <ProtectedRoute>
                <ServiceMapPage />
              </ProtectedRoute>
            }
          />
          <Route
            path="/dashboard/settings"
            element={
              <ProtectedRoute>
                <SettingsPage />
              </ProtectedRoute>
            }
          />

          {/* 404 Catch-all */}
          <Route path="*" element={<NotFoundPage />} />
        </Routes>
      </Suspense>
    </ErrorBoundary>
  )
}

export default App
