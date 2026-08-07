import { Dialog } from '@headlessui/react'
import FlexSearch from 'flexsearch'
import { useMemo, useState } from 'react'
import { useRouter } from 'next/router'
import { flatNavigation } from '@/lib/navigation'

export function Search({ language }) {
  const router = useRouter()
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const docs = useMemo(() => flatNavigation(language), [language])
  const index = useMemo(() => {
    const idx = new FlexSearch.Index({ tokenize: 'forward', cache: true })
    docs.forEach((doc, i) => idx.add(i, `${doc.title} ${doc.section}`))
    return idx
  }, [docs])
  const results = query.trim() ? index.search(query, 8).map(i => docs[i]) : docs.slice(0, 7)
  return <>
    <button onClick={() => setOpen(true)} className="flex w-full items-center justify-between rounded-xl border border-line bg-white/[.035] px-3 py-2 text-left text-sm text-slate-400 hover:border-slate-600">
      <span>{language === 'ru' ? 'Поиск по документации' : 'Search documentation'}</span><kbd className="rounded border border-line px-1.5 text-xs">/</kbd>
    </button>
    <Dialog open={open} onClose={setOpen} className="relative z-50">
      <div className="fixed inset-0 bg-black/70 backdrop-blur-sm" aria-hidden="true" />
      <div className="fixed inset-0 flex items-start justify-center p-4 pt-[12vh]">
        <Dialog.Panel className="w-full max-w-xl overflow-hidden rounded-2xl border border-line bg-[#0d1219] shadow-2xl">
          <input autoFocus value={query} onChange={e => setQuery(e.target.value)} placeholder={language === 'ru' ? 'Начните печатать…' : 'Start typing…'} className="w-full border-b border-line bg-transparent px-5 py-4 text-base text-white outline-none" />
          <div className="max-h-80 overflow-y-auto p-2">
            {results.map(item => <button key={item.href} onClick={() => { setOpen(false); router.push(item.href) }} className="block w-full rounded-xl px-3 py-3 text-left hover:bg-white/[.05]">
              <div className="text-sm font-medium text-white">{item.title}</div><div className="mt-0.5 text-xs text-slate-500">{item.section}</div>
            </button>)}
          </div>
        </Dialog.Panel>
      </div>
    </Dialog>
  </>
}
