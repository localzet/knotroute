import { Dialog } from '@headlessui/react'
import FlexSearch from 'flexsearch'
import { useEffect, useMemo, useState } from 'react'
import { useRouter } from 'next/router'
import { flatNavigation } from '@/lib/navigation'

export function Search({ language }) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const docs = useMemo(() => flatNavigation(language), [language])
  const index = useMemo(() => {
    const idx = new FlexSearch.Index({ tokenize: 'forward', cache: true })
    docs.forEach((doc, i) => idx.add(i, `${doc.title} ${doc.section} ${doc.description || ''}`))
    return idx
  }, [docs])

  useEffect(() => {
    const handler = event => {
      if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') { event.preventDefault(); setOpen(true) }
      if (event.key === '/' && !['INPUT', 'TEXTAREA'].includes(document.activeElement?.tagName)) { event.preventDefault(); setOpen(true) }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [])

  const results = query.trim() ? index.search(query, 10).map(i => docs[i]) : docs.slice(0, 8)
  return <>
    <button onClick={() => setOpen(true)} className="flex w-full items-center justify-between rounded-xl border border-line bg-white/[.035] px-3 py-2 text-left text-sm text-slate-400 hover:border-slate-600">
      <span>{language === 'ru' ? 'Поиск по документации' : 'Search documentation'}</span><kbd className="rounded border border-line px-1.5 text-xs">Ctrl K</kbd>
    </button>
    <Dialog open={open} onClose={setOpen} className="relative z-[60]">
      <div className="fixed inset-0 bg-black/70 backdrop-blur-sm" aria-hidden="true" />
      <div className="fixed inset-0 flex items-start justify-center p-4 pt-[10vh]">
        <Dialog.Panel className="w-full max-w-2xl overflow-hidden rounded-2xl border border-line bg-[#0d1219] shadow-2xl">
          <input autoFocus value={query} onChange={e => setQuery(e.target.value)} placeholder={language === 'ru' ? 'Например: sidecar, Beacon, CA, порт 7447…' : 'For example: sidecar, Beacon, CA, port 7447…'} className="w-full border-b border-line bg-transparent px-5 py-4 text-base text-white outline-none" />
          <div className="max-h-[60vh] overflow-y-auto p-2">
            {results.map(item => <button key={item.href} onClick={() => { setOpen(false); setQuery(''); router.push(item.href) }} className="block w-full rounded-xl px-3 py-3 text-left hover:bg-white/[.05]">
              <div className="text-sm font-medium text-white">{item.title}</div>
              <div className="mt-0.5 text-xs text-slate-500">{item.section} · {item.description}</div>
            </button>)}
            {!results.length ? <div className="p-6 text-center text-sm text-slate-500">{language === 'ru' ? 'Ничего не найдено' : 'No results'}</div> : null}
          </div>
        </Dialog.Panel>
      </div>
    </Dialog>
  </>
}
