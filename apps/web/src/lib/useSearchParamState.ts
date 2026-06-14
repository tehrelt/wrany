import { useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'

/**
 * String state backed by a URL search param, so it survives reloads and can be
 * shared via link. Falls back to `fallback` when the param is absent; setting an
 * empty value removes the param to keep the URL clean.
 */
export function useSearchParamState(
  key: string,
  fallback: string,
): [string, (value: string) => void] {
  const [params, setParams] = useSearchParams()
  const value = params.get(key) ?? fallback

  const setValue = useCallback((next: string) => {
    setParams((prev) => {
      const updated = new URLSearchParams(prev)
      if (next) {
        updated.set(key, next)
      } else {
        updated.delete(key)
      }
      return updated
    }, { replace: true })
  }, [key, setParams])

  return [value, setValue]
}
