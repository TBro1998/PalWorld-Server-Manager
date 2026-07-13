'use client'

import React from 'react'
import { Construction } from 'lucide-react'
import { Card, CardContent } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { useTranslations } from '@/contexts/LanguageContext'

// Shared layout primitives for the server-manage sections. Each section file
// composes these so the reserved areas read consistently while features are
// still stubbed.

// Section wrapper: title + description + a "coming soon" chip.
export function SectionShell({
  title,
  desc,
  children,
}: {
  title: string
  desc: string
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
        <Badge variant="info" className="gap-1.5">
          <Construction className="h-3.5 w-3.5" />
          {t('comingSoon')}
        </Badge>
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
