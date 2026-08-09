import { useQuery } from '@tanstack/react-query'
import { api, isUnauthorized, redirectToLogin } from '../../api/client'

export function AuthGate({ children }: { children: React.ReactNode }) {
  const sessionQuery = useQuery({
    queryKey: ['auth-session'],
    queryFn: api.session,
    retry: false,
  })

  if (sessionQuery.error && isUnauthorized(sessionQuery.error)) {
    redirectToLogin()
    return null
  }

  if (sessionQuery.isLoading) {
    return (
      <main className="grid min-h-screen place-items-center bg-[#f6f8f9] px-4 text-sm text-[#5a6d74]">
        正在验证登录状态...
      </main>
    )
  }

  if (!sessionQuery.data?.authenticated) {
    redirectToLogin()
    return null
  }

  return <>{children}</>
}
