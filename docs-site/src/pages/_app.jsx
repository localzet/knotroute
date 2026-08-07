import 'focus-visible'
import '@/styles/tailwind.css'
import { MDXProvider } from '@mdx-js/react'
import { DocsLayout } from '@/components/DocsLayout'
import { mdxComponents } from '@/components/mdx'

export default function App({ Component, pageProps }) {
  return <MDXProvider components={mdxComponents}><DocsLayout><Component {...pageProps} /></DocsLayout></MDXProvider>
}
