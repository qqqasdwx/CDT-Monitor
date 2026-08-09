import { DashboardPage } from '../../features/dashboard/DashboardPage'
import { AuthGate } from '../shared/AuthGate'
import { renderPage } from '../shared/render'

renderPage(
  <AuthGate>
    <DashboardPage />
  </AuthGate>,
)
