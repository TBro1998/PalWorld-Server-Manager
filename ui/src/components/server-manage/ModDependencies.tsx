import { CheckCircle2, AlertTriangle } from 'lucide-react'
import type { ModDependency } from '@/types/server'

// ModDependencies renders a mod's Info.json dependency list as small badges,
// each marked satisfied (✓, a downloaded global-library mod has that
// PackageName) or missing (⚠). Renders nothing when there are no dependencies.
//
// Shared by the global mod library page (/mods) and the server-scoped mods
// section. Callers pass already-resolved i18n strings so the component stays
// namespace-agnostic.
export function ModDependencies({
  deps,
  label,
  missingLabel,
}: {
  deps: ModDependency[] | null | undefined
  label: string
  missingLabel: string
}) {
  const list = deps ?? []
  if (list.length === 0) return null

  return (
    <div className="mt-1 flex flex-wrap items-center gap-1.5">
      <span className="text-[10px] font-medium text-muted-foreground">{label}:</span>
      {list.map((dep) => (
        <span
          key={dep.name}
          title={dep.satisfied ? undefined : missingLabel}
          className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-[10px] font-medium ${
            dep.satisfied
              ? 'border-success/40 bg-success/10 text-success'
              : 'border-warning/40 bg-warning/10 text-warning'
          }`}
        >
          {dep.satisfied ? (
            <CheckCircle2 size={10} className="shrink-0" />
          ) : (
            <AlertTriangle size={10} className="shrink-0" />
          )}
          {dep.name}
        </span>
      ))}
    </div>
  )
}
