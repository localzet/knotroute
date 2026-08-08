import Head from 'next/head'
import Link from 'next/link'
import { useRouter } from 'next/router'
import { useState } from 'react'
import { Logo } from './Logo'
import { Navigation } from './Navigation'
import { Search } from './Search'
import { MobileNavigation } from './MobileNavigation'
import { TableOfContents } from './TableOfContents'
import { pageNeighbors, flatNavigation } from '@/lib/navigation'

function Breadcrumbs({ language, pathname }) {
  const page = flatNavigation(language).find(item => item.href === pathname)
  if (!page) return null
  return <div className="not-prose mb-5 flex flex-wrap items-center gap-2 text-xs text-slate-600">
    <Link href={`/${language}`} className="hover:text-slate-300">KnotRoute</Link>
    <span>/</span><span>{page.section}</span><span>/</span><span className="text-slate-400">{page.title}</span>
  </div>
}

function PageFooter({ language, pathname }) {
  const { previous, next } = pageNeighbors(language, pathname)
  if (!previous && !next) return null
  return <div className="not-prose mt-16 grid gap-4 border-t border-line pt-8 sm:grid-cols-2">
    {previous ? <Link href={previous.href} className="rounded-2xl border border-line bg-white/[.02] p-4 hover:border-slate-600">
      <div className="text-xs text-slate-600">{language === 'ru' ? '← Назад' : '← Previous'}</div>
      <div className="mt-1 font-medium text-slate-200">{previous.title}</div>
    </Link> : <div />}
    {next ? <Link href={next.href} className="rounded-2xl border border-line bg-white/[.02] p-4 text-right hover:border-slate-600">
      <div className="text-xs text-slate-600">{language === 'ru' ? 'Далее →' : 'Next →'}</div>
      <div className="mt-1 font-medium text-slate-200">{next.title}</div>
    </Link> : null}
  </div>
}

export function DocsLayout({ children }) {
  const router = useRouter()
  const [mobileOpen, setMobileOpen] = useState(false)
  const language = router.pathname.startsWith('/en') ? 'en' : 'ru'
  const alternateLanguage = language === 'ru' ? 'en' : 'ru'
  const alternate = router.asPath.replace(/^\/(ru|en)(?=\/|$)/, `/${alternateLanguage}`)
  const page = flatNavigation(language).find(item => item.href === router.pathname)
  const title = page ? `${page.title} · KnotRoute Docs` : 'KnotRoute Docs'
  const description = page?.description || 'KnotRoute self-hosted encrypted overlay network documentation'

  return <>
    <Head><title>{title}</title><meta name="description" content={description} /><meta name="viewport" content="width=device-width, initial-scale=1" /><script defer src="https://analytics.localzet.com/pixel/RxQXn6zpWPk3kapV" data-ignore-dnt="true"></script></Head>
    <div className="min-h-screen bg-ink text-slate-300">
      <div className="pointer-events-none fixed inset-x-0 top-0 h-72 bg-[radial-gradient(circle_at_50%_-20%,rgba(53,212,125,.17),transparent_58%)]" />
      <header className="sticky top-0 z-40 border-b border-line/80 bg-ink/90 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-[1680px] items-center gap-4 px-4 sm:px-6 lg:px-8">
          <button onClick={() => setMobileOpen(true)} className="rounded-lg border border-line px-2.5 py-1.5 text-sm text-slate-400 lg:hidden" aria-label="Menu">☰</button>
          <Logo language={language} />
          <div className="hidden w-80 lg:block"><Search language={language} /></div>
          <div className="ml-auto flex items-center gap-3">
            <Link href={alternate} className="rounded-lg border border-line px-3 py-1.5 text-xs font-semibold text-slate-400 hover:text-white">{language === 'ru' ? 'EN' : 'RU'}</Link>
            <a href="https://github.com/localzet/knotroute" className="hidden text-sm text-slate-400 hover:text-white sm:inline">GitHub ↗</a>
          </div>
        </div>
      </header>
      <MobileNavigation open={mobileOpen} setOpen={setMobileOpen} language={language} />
      <div className="mx-auto grid max-w-[1680px] grid-cols-1 lg:grid-cols-[290px_minmax(0,1fr)] xl:grid-cols-[290px_minmax(0,1fr)_250px]">
        <aside className="hidden min-h-[calc(100vh-4rem)] border-r border-line px-5 py-7 lg:block">
          <div className="sticky top-24 max-h-[calc(100vh-7rem)] overflow-y-auto pr-1"><Navigation language={language} /></div>
        </aside>
        <main className="min-w-0 px-5 py-8 sm:px-8 lg:px-12 xl:px-14">
          <div className="mx-auto max-w-4xl">
            <Breadcrumbs language={language} pathname={router.pathname} />
            <article className="prose prose-invert max-w-none prose-headings:scroll-mt-24 prose-headings:font-semibold prose-a:text-knot-300 prose-code:text-knot-300 prose-pre:border prose-pre:border-line prose-pre:bg-[#0b1016]">{children}</article>
            <PageFooter language={language} pathname={router.pathname} />
          </div>
        </main>
        <TableOfContents language={language} />
      </div>
    </div>
  </>
}
