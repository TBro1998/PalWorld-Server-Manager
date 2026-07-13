'use client'

import React from 'react'
import { useSearchParams } from 'next/navigation'
import { Construction } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
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
