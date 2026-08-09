import { useMutation, useQuery } from '@tanstack/react-query'
import { LockKeyhole } from 'lucide-react'
import { useState } from 'react'
import { api, ApiError } from '../../api/client'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Input } from '../../components/ui/input'

function nextPath() {
  const params = new URLSearchParams(window.location.search)
  const next = params.get('next')
  if (!next || !next.startsWith('/') || next.startsWith('//') || next.startsWith('/login.html')) {
    return '/dashboard.html'
  }
  return next
}

export function LoginPage() {
  const [password, setPassword] = useState('')
  const sessionQuery = useQuery({
    queryKey: ['auth-session'],
    queryFn: api.session,
    retry: false,
  })
  const login = useMutation({
    mutationFn: api.login,
    onSuccess: () => {
      window.location.assign(nextPath())
    },
  })

  if (sessionQuery.data?.authenticated) {
    window.location.replace(nextPath())
    return null
  }

  const error =
    login.error instanceof ApiError
      ? login.error.message
      : login.error
        ? '登录失败'
        : ''

  return (
    <main className="grid min-h-screen place-items-center bg-[#f6f8f9] px-4 py-10">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <LockKeyhole className="h-4 w-4 text-[#0f766e]" />
            CDT Monitor 登录
          </CardTitle>
        </CardHeader>
        <CardContent>
          <form
            className="grid gap-4"
            onSubmit={(event) => {
              event.preventDefault()
              login.mutate(password)
            }}
          >
            <label className="grid gap-1 text-sm font-medium text-[#263a40]">
              管理员密码
              <Input
                autoFocus
                autoComplete="current-password"
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
              />
            </label>
            {error ? <p className="text-sm text-[#b42318]">{error}</p> : null}
            <Button type="submit" disabled={login.isPending || password.length === 0}>
              登录
            </Button>
          </form>
        </CardContent>
      </Card>
    </main>
  )
}
