import { useEffect, useState } from 'react'

function slugify(value) {
  return value
    .toLowerCase()
    .trim()
    .replace(/[^\p{L}\p{N}\s-]/gu, '')
    .replace(/\s+/g, '-')
}

export function TableOfContents({ language }) {
  const [items, setItems] = useState([])
  const [active, setActive] = useState('')

  useEffect(() => {
    const headings = [...document.querySelectorAll('article h2, article h3')]
    const next = headings.map(heading => {
      if (!heading.id) heading.id = slugify(heading.textContent || '')
      return { id: heading.id, title: heading.textContent || '', level: heading.tagName === 'H3' ? 3 : 2 }
    }).filter(item => item.id)
    setItems(next)

    const observer = new IntersectionObserver(entries => {
      const visible = entries.filter(entry => entry.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
      if (visible[0]) setActive(visible[0].target.id)
    }, { rootMargin: '-96px 0px -72% 0px', threshold: [0, 1] })
    headings.forEach(heading => observer.observe(heading))
    return () => observer.disconnect()
  }, [])

  if (!items.length) return null
  return <aside className="hidden xl:block">
    <div className="sticky top-24 max-h-[calc(100vh-7rem)] overflow-y-auto pl-6 pr-3">
      <div className="mb-3 text-[11px] font-semibold uppercase tracking-[.16em] text-slate-600">
        {language === 'ru' ? 'На этой странице' : 'On this page'}
      </div>
      <nav className="space-y-1 border-l border-line">
        {items.map(item => <a
          key={item.id}
          href={`#${item.id}`}
          className={`block border-l px-3 py-1.5 text-xs leading-5 transition ${item.level === 3 ? 'pl-6' : ''} ${active === item.id ? '-ml-px border-knot-400 text-knot-300' : '-ml-px border-transparent text-slate-500 hover:text-slate-300'}`}
        >{item.title}</a>)}
      </nav>
    </div>
  </aside>
}
