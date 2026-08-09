import { StrictMode, type ReactNode } from 'react'
import { createRoot } from 'react-dom/client'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import '../../index.css'

export function renderPage(page: ReactNode) {
  const root = document.getElementById('root')
  if (!root) {
    throw new Error('missing root element')
  }
  const queryClient = new QueryClient()
  createRoot(root).render(
    <StrictMode>
      <QueryClientProvider client={queryClient}>{page}</QueryClientProvider>
    </StrictMode>,
  )
}
