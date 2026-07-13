import { AxiosError } from 'axios'

// The backend returns errors uniformly as { error: string }. Several REST
// sections surface that message inline (there is no toast library), so the
// extraction lives here as the single source of truth rather than being
// re-derived at each call site.
export function getApiErrorMessage(err: unknown, fallback: string): string {
  if (err instanceof AxiosError) {
    const data = err.response?.data as { error?: string } | undefined
    if (data?.error) return data.error
    if (err.message) return err.message
  }
  if (err instanceof Error && err.message) return err.message
  return fallback
}
