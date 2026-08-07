import Link from 'next/link'

export function Callout({ children, type = 'note', title }) {
  const variants = {
    note: 'border-knot-400/20 bg-knot-400/[.06]',
    warning: 'border-amber-400/25 bg-amber-400/[.07]',
    danger: 'border-red-400/25 bg-red-400/[.07]',
    info: 'border-sky-400/25 bg-sky-400/[.07]',
  }
  return <div className={`not-prose my-6 rounded-2xl border px-5 py-4 text-sm leading-6 text-slate-300 ${variants[type] || variants.note}`}>
    {title ? <div className="mb-1 font-semibold text-white">{title}</div> : null}{children}
  </div>
}

export function Cards({ children }) { return <div className="not-prose my-7 grid gap-4 md:grid-cols-2">{children}</div> }
export function Card({ href, title, children }) { return <Link href={href} className="group rounded-2xl border border-line bg-white/[.025] p-5 transition hover:-translate-y-0.5 hover:border-knot-400/30 hover:bg-knot-400/[.035]"><div className="font-semibold text-white group-hover:text-knot-300">{title} →</div><div className="mt-2 text-sm leading-6 text-slate-400">{children}</div></Link> }
export function Steps({ children }) { return <div className="not-prose my-7 ml-3 border-l border-line pl-7 [counter-reset:step]">{children}</div> }
export function Step({ title, children }) { return <div className="relative mb-8 last:mb-0 before:absolute before:-left-[2.65rem] before:grid before:h-7 before:w-7 before:place-items-center before:rounded-full before:border before:border-knot-400/30 before:bg-[#0d1512] before:text-xs before:font-bold before:text-knot-300 before:[content:counter(step)] [counter-increment:step]"><div className="font-semibold text-white">{title}</div><div className="mt-2 text-sm leading-6 text-slate-400">{children}</div></div> }
export function Badge({ children }) { return <span className="not-prose inline-flex rounded-full border border-knot-400/20 bg-knot-400/[.07] px-2 py-0.5 text-xs font-semibold text-knot-300">{children}</span> }
export function Kbd({ children }) { return <kbd className="not-prose rounded border border-line bg-white/[.04] px-1.5 py-0.5 text-xs text-slate-300">{children}</kbd> }
export function Details({ summary, children }) { return <details className="not-prose my-5 rounded-xl border border-line bg-white/[.02] px-4 py-3"><summary className="cursor-pointer font-medium text-slate-200">{summary}</summary><div className="mt-3 text-sm leading-6 text-slate-400">{children}</div></details> }
export const mdxComponents = { Callout, Cards, Card, Steps, Step, Badge, Kbd, Details }
