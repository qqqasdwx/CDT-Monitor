import { Globe2, RefreshCcw } from 'lucide-react'
import { useState } from 'react'
import { Button } from '../../components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '../../components/ui/card'
import { Input, Select } from '../../components/ui/input'
import type { CloudflareCredential, DNSRecord } from '../../types/api'

export function DDNSPanel({
  credentials,
  records,
  onCreateCredential,
  onCreateRecord,
  onUpdate,
  isSubmitting,
}: {
  credentials: CloudflareCredential[]
  records: DNSRecord[]
  onCreateCredential: (input: { name: string; apiToken: string; enabled: boolean }) => Promise<void>
  onCreateRecord: (input: {
    credentialId: string
    zoneId: string
    recordId: string
    name: string
    type: string
    currentValue: string
    enabled: boolean
  }) => Promise<void>
  onUpdate: (publicIp: string) => void
  isSubmitting: boolean
}) {
  const [credentialName, setCredentialName] = useState('')
  const [apiToken, setAPIToken] = useState('')
  const [credentialId, setCredentialID] = useState('')
  const [zoneId, setZoneID] = useState('')
  const [recordId, setRecordID] = useState('')
  const [recordName, setRecordName] = useState('')
  const [currentValue, setCurrentValue] = useState('')
  const [publicIp, setPublicIP] = useState('')

  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Globe2 className="h-4 w-4" />
          Cloudflare DDNS
        </CardTitle>
      </CardHeader>
      <CardContent className="grid gap-4">
        <form
          className="grid gap-3 md:grid-cols-[1fr_1.4fr_auto]"
          onSubmit={async (event) => {
            event.preventDefault()
            await onCreateCredential({ name: credentialName, apiToken, enabled: true })
            setCredentialName('')
            setAPIToken('')
          }}
        >
          <Input placeholder="凭据名称" value={credentialName} onChange={(event) => setCredentialName(event.target.value)} />
          <Input
            type="password"
            placeholder="Cloudflare API Token"
            value={apiToken}
            onChange={(event) => setAPIToken(event.target.value)}
          />
          <Button type="submit" disabled={isSubmitting}>
            保存凭据
          </Button>
        </form>

        <form
          className="grid gap-3 md:grid-cols-[1fr_1fr_1fr_1fr_auto]"
          onSubmit={async (event) => {
            event.preventDefault()
            await onCreateRecord({
              credentialId,
              zoneId,
              recordId,
              name: recordName,
              type: 'A',
              currentValue,
              enabled: true,
            })
            setZoneID('')
            setRecordID('')
            setRecordName('')
            setCurrentValue('')
          }}
        >
          <Select value={credentialId} onChange={(event) => setCredentialID(event.target.value)}>
            <option value="">选择凭据</option>
            {credentials.map((credential) => (
              <option key={credential.id} value={credential.id}>
                {credential.name}
              </option>
            ))}
          </Select>
          <Input placeholder="Zone ID" value={zoneId} onChange={(event) => setZoneID(event.target.value)} />
          <Input placeholder="Record ID" value={recordId} onChange={(event) => setRecordID(event.target.value)} />
          <Input placeholder="example.com" value={recordName} onChange={(event) => setRecordName(event.target.value)} />
          <Button type="submit" disabled={isSubmitting || credentials.length === 0}>
            保存记录
          </Button>
        </form>

        <div className="grid gap-3 md:grid-cols-[1fr_auto]">
          <Input placeholder="新的公网 IP" value={publicIp} onChange={(event) => setPublicIP(event.target.value)} />
          <Button variant="secondary" onClick={() => onUpdate(publicIp)} disabled={!publicIp}>
            <RefreshCcw className="h-4 w-4" />
            更新 DNS
          </Button>
        </div>

        {records.length === 0 ? (
          <p className="text-sm text-[#687b82]">暂无 DNS 记录</p>
        ) : (
          <div className="grid gap-2">
            {records.map((record) => (
              <div key={record.id} className="rounded-md border border-[#e4eaed] bg-[#fbfcfc] p-3 text-sm">
                <div className="font-medium text-[#172026]">{record.name}</div>
                <p className="mt-1 text-[#40555c]">
                  {record.type} · {record.currentValue || '未设置'} · {record.enabled ? '启用' : '禁用'}
                </p>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
