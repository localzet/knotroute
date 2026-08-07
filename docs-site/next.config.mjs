import createMDX from '@next/mdx'
import remarkGfm from 'remark-gfm'

const withMDX = createMDX({
  options: {
    remarkPlugins: [remarkGfm],
    providerImportSource: '@mdx-js/react',
  },
})

export default withMDX({
  output: 'standalone',
  pageExtensions: ['js', 'jsx', 'md', 'mdx'],
  poweredByHeader: false,
  reactStrictMode: true,
  async redirects() {
    return [{ source: '/', destination: '/ru', permanent: false }]
  },
})
