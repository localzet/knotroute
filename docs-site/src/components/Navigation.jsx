import clsx from 'clsx'
import Link from 'next/link'
import { useRouter } from 'next/router'
import { navigation } from '@/lib/navigation'

export function Navigation({ language }) {
  const router = useRouter()
  return <nav className="space-y-7">
    {(navigation[language] || navigation.en).map(section => <div key={section.title}>
      <div className="mb-2 px-2 text-[11px] font-semibold uppercase tracking-[.18em] text-slate-600">{section.title}</div>
      <div className="space-y-1">{section.links.map(link => <Link key={link.href} href={link.href} className={clsx('block rounded-lg px-2 py-1.5 text-sm transition', router.pathname === link.href ? 'bg-knot-400/10 text-knot-300' : 'text-slate-400 hover:bg-white/[.035] hover:text-white')}>{link.title}</Link>)}</div>
    </div>)}
  </nav>
}
