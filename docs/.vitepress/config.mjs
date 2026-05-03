import { defineConfig } from 'vitepress'
import { withMermaid } from 'vitepress-plugin-mermaid'

// https://vitepress.dev/reference/site-config
export default withMermaid(defineConfig({
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
          { text: 'The DX Landscape', link: '/guide/landscape' },
          { text: 'Getting Started', link: '/guide/getting-started' },
          { text: 'User Journeys', link: '/guide/journeys' },
          { text: 'Architecture', link: '/guide/architecture' },
          { text: 'AI Agent Skills', link: '/guide/ai-agents' }
        ]
      },
      {
        text: 'Local Infrastructure',
        items: [
          { text: 'Container VMs', link: '/guide/virtual-machine' },
          { text: 'VM Providers', link: '/guide/providers' },
          { text: 'Databases (Ephemeral)', link: '/guide/databases' },
          { text: 'Cloud Emulators (GCS)', link: '/guide/cloud-emulators' },
          { text: 'The Nuke Button', link: '/guide/nuke' }
        ]
      },
      {
        text: 'Kubernetes & Hybrid',
        items: [
          { text: 'Hybrid Bridge', link: '/guide/bridge' },
          { text: 'Zero-Config Kubernetes', link: '/guide/kubernetes' },
          { text: 'Multi-Node Clusters', link: '/guide/multinode' }
        ]
      },
      {
        text: 'Networking & Edge',
        items: [
          { text: 'Cloudflare Tunnels', link: '/guide/tunnels' },
          { text: 'Email Catcher', link: '/guide/mail' },
          { text: 'Webhook Catcher', link: '/guide/webhook' }
        ]
      },
      {
        text: 'Orchestration & State',
        items: [
          { text: 'Multirepo Orchestration', link: '/guide/multirepo' },
          { text: 'Vault Secrets Syncing', link: '/guide/vaults' },
          { text: 'Smart File Syncing', link: '/guide/sync' },
          { text: 'Native Apps & Logs', link: '/guide/execution' },
          { text: 'PR Preview Sandbox', link: '/guide/preview' },
          { text: 'Diagnostics & State', link: '/guide/state' }
        ]
      },
      {
        text: 'Testing & Telemetry',
        items: [
          { text: 'API Mocking', link: '/guide/mocking' },
          { text: 'Ephemeral E2E Testing', link: '/guide/testing' },
          { text: 'Security Auditing', link: '/guide/audit' },
          { text: 'Distributed Tracing', link: '/guide/trace' },
          { text: 'Environment Doctor', link: '/guide/doctor' }
        ]
      },
      {
        text: 'Pipelines & CI/CD',
        items: [
          { text: 'Pipeline Stages', link: '/guide/pipeline' },
          { text: 'Local CI Emulation', link: '/guide/ci' },
          { text: 'Predictive Pre-Building', link: '/guide/caching' }
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
}))
