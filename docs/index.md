---
# https://vitepress.dev/reference/default-theme-home-page
layout: home

hero:
  name: "devx"
  text: "Advanced Agentic Coding Environment Orchestrator"
  tagline: "Unifying Podman VMs, Ephemeral Databases, Cloudflare Tunnels, and AI Agents."
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/VitruvianSoftware/devx

features:
  - title: Ephemeral Virtual Machines
    details: Instantaneously spawns isolated, high-performance Podman Linux VMs, bypassing complex hypervisor configurations.
  - title: Single-Command Database Spawning
    details: Need a Postgres, MySQL, or Redis instance? `devx db spawn postgres` provisions and maps it instantly, bypassing heavy Kubernetes layers.
  - title: Native Ngrok-Style Tunneling
    details: Seamlessly map localhost HTTP surfaces into secure Cloudflare Tunnels. Spin up shareable internet links without managing DNS.
  - title: Agent Ready
    details: The CLI is entirely orchestrable by AI Agents. Outputs deterministic JSON, supports deterministic Exit Codes, and installs contextual Agent Skills for Claude, Copilot, and Cursor out of the box.
---
