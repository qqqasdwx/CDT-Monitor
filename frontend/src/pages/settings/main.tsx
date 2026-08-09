import { AuthGate } from '../shared/AuthGate'
import { renderPage } from '../shared/render'
import { SettingsPage } from './SettingsPage'

renderPage(
  <AuthGate>
    <SettingsPage />
  </AuthGate>,
)
