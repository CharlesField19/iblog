export interface InitialData {
  __kind?: string
  __path?: string
  article?: unknown
  items?: unknown[]
  page?: number
  pageSize?: number
  total?: number
  settings?: {
    site?: Record<string, unknown>
    contacts?: Record<string, string>
    about?: Record<string, unknown>
  }
}

let cached: InitialData | null | undefined
const consumed = new Set<string>()

export function readInitialData<T extends InitialData = InitialData>(): T | null {
  if (cached === undefined) {
    const node = document.getElementById('__IBLOG_INITIAL__')
    if (!node?.textContent) {
      cached = null
    } else {
      try {
        cached = JSON.parse(node.textContent) as T
      } catch {
        cached = null
      }
    }
  }
  if (cached?.__path && cached.__path !== `${window.location.pathname}${window.location.search}`)
    return null
  return cached as T | null
}

export function takeInitialData<T extends InitialData = InitialData>(kind: string): T | null {
  const data = readInitialData<T>()
  if (!data || data.__kind !== kind || consumed.has(kind)) return null
  consumed.add(kind)
  return data
}
