import { useCallback } from 'react'
import { useSearchParams } from 'react-router-dom'

/**
 * Persists the currently selected entity id in the URL search params under
 * `key`, so the selection survives reloads and can be shared via link.
 * Passing `null` removes the param to keep the URL clean.
 */
export function useSelectedId(key: string): [string | null, (id: string | null) => void] {
  const [params, setParams] = useSearchParams()
  const selected = params.get(key)

  const setSelected = useCallback((id: string | null) => {
    setParams((prev) => {
      const updated = new URLSearchParams(prev)
      if (id === null) {
        updated.delete(key)
      } else {
        updated.set(key, id)
      }
      return updated
    }, { replace: true })
  }, [key, setParams])

  return [selected, setSelected]
}
