import { Dialog } from '@headlessui/react'
import { Navigation } from './Navigation'
import { Search } from './Search'

export function MobileNavigation({ open, setOpen, language }) {
  return <Dialog open={open} onClose={setOpen} className="relative z-50 lg:hidden">
    <div className="fixed inset-0 bg-black/70 backdrop-blur-sm" aria-hidden="true" />
    <div className="fixed inset-y-0 left-0 w-[min(88vw,340px)] overflow-y-auto border-r border-line bg-[#0b0f15] p-5 shadow-2xl">
      <div className="mb-5 flex items-center justify-between">
        <div className="text-sm font-semibold text-white">KnotRoute Docs</div>
        <button onClick={() => setOpen(false)} className="rounded-lg border border-line px-2.5 py-1.5 text-xs text-slate-400">×</button>
      </div>
      <Search language={language}/>
      <div className="mt-7" onClick={event => { if (event.target.closest('a')) setOpen(false) }}><Navigation language={language}/></div>
    </div>
  </Dialog>
}
