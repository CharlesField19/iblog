export type ArticleStatus = 'draft' | 'published'

export interface ArticleSummary {
  id: number
  slug: string
  title: string
  summary: string
  status: ArticleStatus
  views: number
  createdAt: string
  updatedAt: string
  publishedAt?: string | null
  tags: string[]
}

export interface ArticleDetail extends ArticleSummary {
  contentMd?: string
  contentHtml?: string
  previous?: { slug: string; title: string } | null
  next?: { slug: string; title: string } | null
}

export interface PageResult<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

export interface Tag {
  id: number
  name: string
  count: number
}

export type PublicTag = Pick<Tag, 'name' | 'count'>

export interface Friend {
  id: number
  name: string
  url: string
  avatar: string
  description: string
  sortOrder: number
}

export interface TimelineItem {
  year: string
  title: string
  description: string
}

export interface AboutData {
  intro: string
  bio: string
  skills: string[]
  timeline: TimelineItem[]
  contactNote: string
}

export interface SiteData {
  titlePrefix: string
  titleAccent: string
  subtitle: string
  footerName: string
  footerText: string
  email: string
  socials: Record<string, string>
}

export type ToastKind = 'success' | 'error' | 'info'
export type ToastFunction = (kind: ToastKind, text: string) => void
export type ConfirmFunction = (message: string, title?: string) => Promise<boolean>

export interface SessionResponse {
  expiresAt: string
  username?: string
}

export interface PublicSettings {
  site: Partial<SiteData> & { name?: string }
  contacts: Record<string, string>
  about: AboutData
}

export interface AdminSettings {
  settings: Partial<Omit<PublicSettings, 'about'>> & { about?: Partial<AboutData> }
}
