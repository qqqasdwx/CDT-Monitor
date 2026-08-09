import { Trash2 } from 'lucide-react'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { formatBytes, formatTime } from '../../lib/utils'
import type { InstanceStatusPoint, TrafficPoint } from '../../types/api'

export function HistoryPanel({
  traffic,
  thresholdBytes,
  statusHistory,
  onCleanup,
  cleanupPending,
}: {
  traffic: TrafficPoint[]
  thresholdBytes: number
  statusHistory: InstanceStatusPoint[]
  onCleanup: () => void
  cleanupPending: boolean
}) {
  return (
    <div className="grid gap-5 xl:grid-cols-[1.2fr_0.8fr]">
      <TrafficChart points={traffic} thresholdBytes={thresholdBytes} />
      <StatusHistory points={statusHistory} onCleanup={onCleanup} cleanupPending={cleanupPending} />
    </div>
  )
}

function TrafficChart({ points, thresholdBytes }: { points: TrafficPoint[]; thresholdBytes: number }) {
  const max = Math.max(...points.map((point) => point.totalBytes), thresholdBytes, 1)
  const width = 520
  const height = 180
  const path = points
    .map((point, index) => {
      const x = points.length <= 1 ? 0 : (index / (points.length - 1)) * width
      const y = height - (point.totalBytes / max) * height
      return `${index === 0 ? 'M' : 'L'} ${x.toFixed(1)} ${y.toFixed(1)}`
    })
    .join(' ')

  return (
    <Card>
      <CardHeader>
        <CardTitle>流量趋势</CardTitle>
      </CardHeader>
      <CardContent>
        {points.length === 0 ? (
          <p className="text-sm text-[#687b82]">暂无流量历史</p>
        ) : (
          <div className="grid gap-3">
            <svg viewBox={`0 0 ${width} ${height}`} className="h-48 w-full overflow-visible">
              {thresholdBytes > 0 ? (
                <line
                  x1="0"
                  x2={width}
                  y1={height - (thresholdBytes / max) * height}
                  y2={height - (thresholdBytes / max) * height}
                  stroke="#b42318"
                  strokeDasharray="6 6"
                  strokeWidth="2"
                />
              ) : null}
              <path d={path} fill="none" stroke="#0f766e" strokeWidth="3" strokeLinecap="round" />
              {points.map((point, index) => {
                const x = points.length <= 1 ? 0 : (index / (points.length - 1)) * width
                const y = height - (point.totalBytes / max) * height
                return <circle key={`${point.bucketStart}-${index}`} cx={x} cy={y} r="3" fill="#134e4a" />
              })}
            </svg>
            <div className="flex flex-wrap justify-between gap-2 text-xs text-[#687b82]">
              <span>{formatTime(points[0]?.bucketStart)}</span>
              <span>峰值 {formatBytes(max)}</span>
              {thresholdBytes > 0 ? <span>阈值 {formatBytes(thresholdBytes)}</span> : null}
              <span>{formatTime(points[points.length - 1]?.bucketStart)}</span>
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function StatusHistory({
  points,
  onCleanup,
  cleanupPending,
}: {
  points: InstanceStatusPoint[]
  onCleanup: () => void
  cleanupPending: boolean
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-3">
        <CardTitle>状态历史</CardTitle>
        <Button variant="secondary" onClick={onCleanup} disabled={cleanupPending}>
          <Trash2 className="h-4 w-4" />
          清理日志
        </Button>
      </CardHeader>
      <CardContent className="grid gap-3">
        {points.length === 0 ? (
          <p className="text-sm text-[#687b82]">暂无状态历史</p>
        ) : (
          points.slice(0, 8).map((point) => (
            <div key={point.id} className="rounded-md border border-[#e4eaed] bg-[#fbfcfc] p-3 text-sm">
              <div className="flex flex-wrap justify-between gap-2">
                <span className="font-medium text-[#172026]">{point.instanceId}</span>
                <span className="text-xs text-[#687b82]">{formatTime(point.observedAt)}</span>
              </div>
              <p className="mt-1 text-[#40555c]">{point.status}</p>
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}
