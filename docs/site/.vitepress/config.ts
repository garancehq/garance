import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Garance',
  description: 'The open source Backend-as-a-Service',
  ignoreDeadLinks: [
    /localhost/,
  ],
  themeConfig: {
    nav: [
      { text: 'Docs', link: '/getting-started' },
      { text: 'API', link: '/api' },
      { text: 'GitHub', link: 'https://github.com/garancehq/garance' },
    ],
    sidebar: [
      {
        text: 'Introduction',
        items: [
          { text: 'Getting Started', link: '/getting-started' },
          { text: 'Self-Hosting', link: '/self-host' },
        ],
      },
      {
        text: 'Features',
        items: [
          { text: 'Schema DSL', link: '/schema' },
          { text: 'Authentication', link: '/auth' },
          { text: 'Storage', link: '/storage' },
        ],
      },
      {
        text: 'Client',
        items: [
          { text: 'TypeScript SDK', link: '/sdk' },
          { text: 'CLI', link: '/cli' },
          { text: 'REST API', link: '/api' },
        ],
      },
    ],
    socialLinks: [
      { icon: 'github', link: 'https://github.com/garancehq/garance' },
    ],
    footer: {
      message: 'Released under the Apache 2.0 License.',
    },
    search: {
      provider: 'local',
    },
  },
})
