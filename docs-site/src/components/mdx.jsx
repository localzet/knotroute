import Link from 'next/link'

export function Callout({ children, type = 'note' }) {
  const classes = type === 'warning' ? 'border-amber-400/25 bg-amber-400/[.07]' : 'border-knot-400/20 bg-knot-400/[.06]'
  return <div className={`my-6 rounded-2xl border px-5 py-4 text-sm ${classes}`}>{children}</div>
}
export function Cards({ children }) { return <div className="not-prose my-7 grid gap-4 md:grid-cols-2">{children}</div> }
export function Card({ href, title, children }) { return <Link href={href} className="group rounded-2xl border border-line bg-white/[.025] p-5 transition hover:-translate-y-0.5 hover:border-knot-400/30 hover:bg-knot-400/[.035]"><div className="font-semibold text-white group-hover:text-knot-300">{title} →</div><div className="mt-2 text-sm leading-6 text-slate-400">{children}</div></Link> }
export const mdxComponents = { Callout, Cards, Card }
