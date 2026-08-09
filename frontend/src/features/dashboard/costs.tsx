import { CreditCard, RefreshCcw } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { formatTime } from '../../lib/utils'
import type { CostSnapshot } from '../../types/api'

export function CostPanel({
  costs,
  onSync,
  isSyncing,
}: {
  costs: CostSnapshot[]
  onSync: () => void
  isSyncing: boolean
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-3">
        <CardTitle className="flex items-center gap-2">
          <CreditCard className="h-4 w-4" />
          费用状态
        </CardTitle>
        <Button variant="secondary" onClick={onSync} disabled={isSyncing}>
          <RefreshCcw className="h-4 w-4" />
          同步费用
        </Button>
      </CardHeader>
      <CardContent className="grid gap-3">
        {costs.length === 0 ? (
          <p className="text-sm text-[#687b82]">暂无费用快照</p>
        ) : (
          costs.map((snapshot) => (
            <div key={snapshot.id} className="rounded-md border border-[#e4eaed] bg-[#fbfcfc] p-3 text-sm">
              <div className="flex flex-wrap justify-between gap-2">
                <span className="font-medium text-[#172026]">
                  {snapshot.accountName || snapshot.accountId}
                </span>
                <span className="text-xs text-[#687b82]">{formatTime(snapshot.collectedAt)}</span>
              </div>
              <p className="mt-1 text-[#40555c]">
                可用余额 {snapshot.availableAmount.toFixed(2)} {snapshot.currency} · 信用额度{' '}
                {snapshot.creditAmount.toFixed(2)} {snapshot.currency}
              </p>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}
