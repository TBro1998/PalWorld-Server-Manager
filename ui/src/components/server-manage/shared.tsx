'use client'

import React, { useState } from 'react'
import { useSearchParams } from 'next/navigation'
import { Construction, Eye, EyeOff } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { useQuery } from '@tanstack/react-query'
import { serversApi } from '@/lib/api'
import { useTranslations } from '@/contexts/LanguageContext'

// Reads the ?id= query param the manage panel routes on and returns it as a
// number (NaN when absent/invalid). The manage page already wraps everything in
// a Suspense boundary, so sections may call useSearchParams directly. Sections
// receive no props, so each resolves the server id through this single helper.
export function useServerId(): number {
  const searchParams = useSearchParams()
  const raw = searchParams.get('id')
  return raw ? Number(raw) : NaN
}

// Shared server query. Sections that need the full server object (status,
// installed, launch_args) call this instead of taking props; the ['server', id]
// key is deduped with the manage page's own query. Polls every 5s to keep
// status fresh, matching the manage page.
export function useServer() {
  const serverId = useServerId()
  return useQuery({
    queryKey: ['server', serverId],
    queryFn: async () => (await serversApi.get(serverId)).data,
    enabled: Number.isFinite(serverId),
    refetchInterval: 5000,
  })
}

// Shared layout primitives for the server-manage sections. Each section file
// composes these so the reserved areas read consistently while features are
// still stubbed.

// Section wrapper: title + description + an optional "coming soon" chip.
// comingSoon defaults to true for the still-stubbed sections; functional
// sections (Overview/Players/Operations) pass false so the chip does not
// mislabel live features.
export function SectionShell({
  title,
  desc,
  comingSoon = true,
  children,
}: {
  title: string
  desc: string
  comingSoon?: boolean
  children: React.ReactNode
}) {
  const t = useTranslations('serverManage')
  return (
    <div className="space-y-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-xl font-bold text-foreground">{title}</h2>
          <p className="mt-1 text-sm text-muted-foreground">{desc}</p>
        </div>
        {comingSoon && (
          <Badge variant="info" className="gap-1.5">
            <Construction className="h-3.5 w-3.5" />
            {t('comingSoon')}
          </Badge>
        )}
      </div>
      {children}
    </div>
  )
}

// A dashed placeholder region standing in for not-yet-built content.
export function Placeholder({
  className = '',
  children,
}: {
  className?: string
  children?: React.ReactNode
}) {
  return (
    <div
      className={
        'flex items-center justify-center rounded-2xl border-2 border-dashed border-border/70 bg-muted/30 p-6 text-center text-sm text-muted-foreground ' +
        className
      }
    >
      {children}
    </div>
  )
}

// PasswordInput is a text input masked by default with a show/hide toggle.
// Shared by the Basics settings page and the Steam login form.
export function PasswordInput({
  id,
  value,
  onChange,
}: {
  id?: string
  value: string
  onChange: (v: string) => void
}) {
  const [show, setShow] = useState(false)
  return (
    <div className="relative">
      <Input
        id={id}
        type={show ? 'text' : 'password'}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="pr-9"
      />
      <button
        type="button"
        tabIndex={-1}
        onClick={() => setShow((s) => !s)}
        className="absolute inset-y-0 right-0 flex items-center px-2.5 text-muted-foreground hover:text-foreground"
      >
        {show ? <EyeOff size={16} /> : <Eye size={16} />}
      </button>
    </div>
  )
}

// LaunchToggle / LaunchNumber: label + control rows used by the Launch settings
// page (and the port row on Basics).
export function LaunchToggle({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (c: boolean) => void
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
      <Label>{label}</Label>
      <Switch checked={checked} onCheckedChange={onChange} />
    </div>
  )
}

export function LaunchNumber({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string
  value: number | undefined
  onChange: (v: number | undefined) => void
  placeholder?: string
}) {
  return (
    <div className="flex items-center justify-between gap-4 border-b border-dashed pb-2">
      <Label>{label}</Label>
      <Input
        type="number"
        value={value ?? ''}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
        className="max-w-[220px]"
      />
    </div>
  )
}

// A titled card used to frame a reserved sub-area within a section.
export function PanelCard({
  icon,
  title,
  children,
}: {
  icon: React.ReactNode
  title: string
  children: React.ReactNode
}) {
  return (
    <Card className="rounded-2xl border-2 shadow-pal">
      <CardContent className="space-y-3 p-5">
        <div className="flex items-center gap-2 text-foreground">
          <span className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
            {icon}
          </span>
          <h3 className="font-bold">{title}</h3>
        </div>
        {children}
      </CardContent>
    </Card>
  )
}
