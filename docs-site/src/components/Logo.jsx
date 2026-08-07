import Link from 'next/link'

export function Logo({ language = 'ru' }) {
  return <Link href={`/${language}`} className="group flex items-center gap-3 font-semibold text-white">
    <span className="grid h-9 w-9 place-items-center rounded-xl border border-knot-400/30 bg-knot-400/10 shadow-glow">
      <svg viewBox="0 0 32 32" className="h-5 w-5 fill-none stroke-knot-300" strokeWidth="2.4">
        <path d="M8 8c4-4 12-4 16 0s4 12 0 16-12 4-16 0-4-12 0-16Z" />
        <path d="m10 21 12-10M9 12l11 11" />
      </svg>
    </span>
    <span>KnotRoute <span className="text-slate-500">Docs</span></span>
  </Link>
}
