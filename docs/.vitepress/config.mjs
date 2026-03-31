import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "devx",
  description: "Advanced Agentic Coding Environment Orchestrator",
  server: { port: 5173 },
  themeConfig: {
    // https://vitepress.dev/reference/default-theme-config
    logo: 'https://github.com/VitruvianSoftware.png',
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Documentation', link: '/guide/getting-started' }
    ],

    sidebar: [
      {
        text: 'Introduction',
        items: [
          { text: 'What is devx?', link: '/guide/introduction' },
          { text: 'Getting Started', link: '/guide/getting-started' },
          { text: 'Environment Doctor', link: '/guide/doctor' },
          { text: 'Architecture', link: '/guide/architecture' }
        ]
      },
      {
        text: 'Core Concepts',
        items: [
          { text: 'Container VMs', link: '/guide/virtual-machine' },
          { text: 'Databases (Ephemeral)', link: '/guide/databases' },
          { text: 'Cloudflare Tunnels', link: '/guide/tunnels' },
          { text: 'AI Agent Skills', link: '/guide/ai-agents' }
        ]
      }
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/VitruvianSoftware/devx' }
    ],
    
    footer: {
      message: 'Released under the MIT License.',
      copyright: 'Copyright © 2026-present VitruvianSoftware'
    }
  }
})
