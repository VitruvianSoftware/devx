import { defineConfig } from 'vitepress'

// https://vitepress.dev/reference/site-config
export default defineConfig({
  title: "devx",
  description: "Advanced Agentic Coding Environment Orchestrator",
  server: { port: 5173 },
  // localhost URLs in docs are intentional (they're local dev addresses, not broken links)
  ignoreDeadLinks: [/^http:\/\/localhost/],
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
          { text: 'User Journeys', link: '/guide/journeys' },
          { text: 'Environment Doctor', link: '/guide/doctor' },
          { text: 'Architecture', link: '/guide/architecture' },
          { text: 'Diagnostics & State', link: '/guide/state' }
        ]
      },
      {
        text: 'Core Concepts',
        items: [
          { text: 'Container VMs', link: '/guide/virtual-machine' },
          { text: 'VM Providers', link: '/guide/providers' },
          { text: 'Databases (Ephemeral)', link: '/guide/databases' },
          { text: 'Cloudflare Tunnels', link: '/guide/tunnels' },
          { text: 'Native Apps & Logs', link: '/guide/execution' },
          { text: 'Cloud Emulators (GCS)', link: '/guide/cloud-emulators' },
          { text: 'The Nuke Button', link: '/guide/nuke' },
          { text: 'Email Catcher', link: '/guide/mail' },
          { text: 'Webhook Catcher', link: '/guide/webhook' },
          { text: 'Security Auditing', link: '/guide/audit' },
          { text: 'Distributed Tracing', link: '/guide/trace' },
          { text: 'Ephemeral E2E Testing', link: '/guide/testing' },
          { text: 'API Mocking', link: '/guide/mocking' },
          { text: 'Zero-Config Kubernetes', link: '/guide/kubernetes' },
          { text: 'Homelab Manager', link: '/guide/homelab' },
          { text: 'Vault Secrets Syncing', link: '/guide/vaults' },
          { text: 'Local CI Emulation', link: '/guide/ci' },
          { text: 'Smart File Syncing', link: '/guide/sync' },
          { text: 'Multirepo Orchestration', link: '/guide/multirepo' },
          { text: 'Predictive Pre-Building', link: '/guide/caching' },
          { text: 'Pipeline Stages', link: '/guide/pipeline' },
          { text: 'AI Agent Skills', link: '/guide/ai-agents' },
          { text: 'Hybrid Bridge', link: '/guide/bridge' }
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
