import Head from 'next/head'
import Link from 'next/link'
import { useRouter } from 'next/router'
import { Logo } from './Logo'
import { Navigation } from './Navigation'
import { Search } from './Search'

export function DocsLayout({ children }) {
  const router = useRouter()
  const language = router.pathname.startsWith('/en') ? 'en' : 'ru'
  const alternate = router.asPath.replace(/^\/(ru|en)/, `/${language === 'ru' ? 'en' : 'ru'}/`).replace(/^\/(ru|en)$/, `/${language === 'ru' ? 'en' : 'ru'}`)
  return <>
    <Head><title>KnotRoute Docs</title><meta name="description" content="KnotRoute self-hosted encrypted overlay network documentation" /></Head>
    <div className="min-h-screen bg-ink text-slate-300">
      <div className="pointer-events-none fixed inset-x-0 top-0 h-64 bg-[radial-gradient(circle_at_50%_-20%,rgba(53,212,125,.18),transparent_58%)]" />
      <header className="sticky top-0 z-40 border-b border-line/80 bg-ink/85 backdrop-blur-xl">
        <div className="mx-auto flex h-16 max-w-[1500px] items-center gap-5 px-5 lg:px-8"><Logo language={language}/><div className="ml-auto flex items-center gap-3"><Link href={alternate} className="rounded-lg border border-line px-3 py-1.5 text-xs font-semibold text-slate-400 hover:text-white">{language === 'ru' ? 'EN' : 'RU'}</Link><a href="https://github.com/localzet/knotroute" className="text-sm text-slate-400 hover:text-white">GitHub ↗</a></div></div>
      </header>
      <div className="mx-auto grid max-w-[1500px] grid-cols-1 lg:grid-cols-[280px_minmax(0,1fr)]">
        <aside className="hidden min-h-[calc(100vh-4rem)] border-r border-line px-5 py-7 lg:block"><Search language={language}/><div className="mt-7"><Navigation language={language}/></div></aside>
        <main className="min-w-0 px-5 py-9 sm:px-8 lg:px-12 xl:px-16"><div className="mx-auto max-w-4xl"><div className="mb-7 lg:hidden"><Search language={language}/></div><article className="prose prose-invert max-w-none prose-headings:scroll-mt-24 prose-headings:font-semibold prose-a:text-knot-300 prose-code:text-knot-300 prose-pre:border prose-pre:border-line prose-pre:bg-[#0b1016]">{children}</article></div></main>
      </div>
    </div>
  </>
}
