# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.58.0](https://github.com/VitruvianSoftware/devx/compare/v0.57.0...v0.58.0) (2026-05-29)


### Features

* **config:** inject KUBECONFIG into run/action child commands from devx.yaml ([#231](https://github.com/VitruvianSoftware/devx/issues/231)) ([dc53dba](https://github.com/VitruvianSoftware/devx/commit/dc53dba6f497dec847c3064f83900fc4a02025a6))


### Bug Fixes

* **agent:** handle already-committed branches in review/ship ([#230](https://github.com/VitruvianSoftware/devx/issues/230)) ([227addf](https://github.com/VitruvianSoftware/devx/commit/227addff76957e72e30a8d12a7a86b33aa2d18e1))

## [0.57.0](https://github.com/VitruvianSoftware/devx/compare/v0.56.0...v0.57.0) (2026-05-29)


### Features

* **cluster:** host workspace mounts for multi-node Lima VMs ([#227](https://github.com/VitruvianSoftware/devx/issues/227)) ([4803ca0](https://github.com/VitruvianSoftware/devx/commit/4803ca0a204923320a49e676a3484c8c64d523cf))


### Bug Fixes

* **agent:** steer agents to 'review' (open PR) vs 'ship' (merge) in push guardrail ([#228](https://github.com/VitruvianSoftware/devx/issues/228)) ([6afbaf3](https://github.com/VitruvianSoftware/devx/commit/6afbaf366fa7907bc264d62faae4656672cc09da))

## [0.56.0](https://github.com/VitruvianSoftware/devx/compare/v0.55.0...v0.56.0) (2026-05-24)


### Features

* **up:** opt-in log streaming across runtimes + container deployer ([#224](https://github.com/VitruvianSoftware/devx/issues/224)) ([27a9820](https://github.com/VitruvianSoftware/devx/commit/27a98200cf6799b96938b1f95ad288056268a6e1))


### Bug Fixes

* **docs:** exclude superpowers/ design specs from VitePress build ([#226](https://github.com/VitruvianSoftware/devx/issues/226)) ([72d5430](https://github.com/VitruvianSoftware/devx/commit/72d5430794120e95658f24ab53a891dfaf4d0151))

## [0.55.0](https://github.com/VitruvianSoftware/devx/compare/v0.54.0...v0.55.0) (2026-05-24)


### Features

* **orchestrator:** print service access URLs on up + teardown summary on exit ([#220](https://github.com/VitruvianSoftware/devx/issues/220)) ([9350682](https://github.com/VitruvianSoftware/devx/commit/93506821ddecad5d271dcbf62d9ef61b016b3af9))

## [0.54.0](https://github.com/VitruvianSoftware/devx/compare/v0.53.0...v0.54.0) (2026-05-23)


### Features

* **orchestrator:** add one-shot (run-to-completion) tasks ([#213](https://github.com/VitruvianSoftware/devx/issues/213)) ([f6fb72f](https://github.com/VitruvianSoftware/devx/commit/f6fb72fa9109adfed1c1f5ce2652c76743300676))

## [0.53.0](https://github.com/VitruvianSoftware/devx/compare/v0.52.0...v0.53.0) (2026-05-23)


### Features

* **orchestrator:** add automatic port-forward discovery for runtime: kubernetes ([#211](https://github.com/VitruvianSoftware/devx/issues/211)) ([83004a4](https://github.com/VitruvianSoftware/devx/commit/83004a414a58ec49438fd806945dadf38f47f16e))

## [0.52.0](https://github.com/VitruvianSoftware/devx/compare/v0.51.0...v0.52.0) (2026-05-23)


### Features

* **orchestrator:** add Cloud Run deployer for runtime: cloud ([#209](https://github.com/VitruvianSoftware/devx/issues/209)) ([4f34ec8](https://github.com/VitruvianSoftware/devx/commit/4f34ec81cbc0789c0299a1826afeed29f5081066))

## [0.51.0](https://github.com/VitruvianSoftware/devx/compare/v0.50.0...v0.51.0) (2026-05-23)


### Features

* **orchestrator:** add live-reload (pod file sync) for runtime: kubernetes ([#207](https://github.com/VitruvianSoftware/devx/issues/207)) ([f113782](https://github.com/VitruvianSoftware/devx/commit/f11378295879db027e57b627edb9ccc6b39d5abc))

## [0.50.0](https://github.com/VitruvianSoftware/devx/compare/v0.49.0...v0.50.0) (2026-05-23)


### Features

* **orchestrator:** add helm renderer for runtime: kubernetes ([#205](https://github.com/VitruvianSoftware/devx/issues/205)) ([a8e2c81](https://github.com/VitruvianSoftware/devx/commit/a8e2c815c883788457af2a15027c2b94d29b4f15))

## [0.49.0](https://github.com/VitruvianSoftware/devx/compare/v0.48.0...v0.49.0) (2026-05-23)


### Features

* **image:** build + load service images into the cluster for runtime: kubernetes ([#203](https://github.com/VitruvianSoftware/devx/issues/203)) ([f50ca73](https://github.com/VitruvianSoftware/devx/commit/f50ca7391a304b10570135c45c837aa855711969))

## [0.48.0](https://github.com/VitruvianSoftware/devx/compare/v0.47.0...v0.48.0) (2026-05-23)


### Features

* **orchestrator:** implement runtime: kubernetes cluster deploy ([#201](https://github.com/VitruvianSoftware/devx/issues/201)) ([b67fa0f](https://github.com/VitruvianSoftware/devx/commit/b67fa0f15747e12fe05a1e80c8498c9b9e8ce84f))

## [0.47.0](https://github.com/VitruvianSoftware/devx/compare/v0.46.0...v0.47.0) (2026-05-23)


### Features

* support Docker container runtime and socket forwarding on multi-node clusters ([1e3bd28](https://github.com/VitruvianSoftware/devx/commit/1e3bd28911ee88234027cae5393c605de3f2eb81))

## [0.46.0](https://github.com/VitruvianSoftware/devx/compare/v0.45.0...v0.46.0) (2026-05-18)


### Features

* **cron:** add devx cron list/run for local cron-job testing (Idea 66) ([#197](https://github.com/VitruvianSoftware/devx/issues/197)) ([0cf55e2](https://github.com/VitruvianSoftware/devx/commit/0cf55e289167eb65f87b9578dd15f58f13e5bc0f))

## [0.45.0](https://github.com/VitruvianSoftware/devx/compare/v0.44.0...v0.45.0) (2026-05-17)


### Features

* add zero-config auto-detection for Bazel stacks ([11f40b6](https://github.com/VitruvianSoftware/devx/commit/11f40b6f6a569f9e21ba7c4eac780ba4f1773bbe))
* **mcp:** devx as a first-class MCP tool surface for AI coding agents ([#182](https://github.com/VitruvianSoftware/devx/issues/182)) ([c651580](https://github.com/VitruvianSoftware/devx/commit/c651580938052a51b75c75ac2e116c1e65ea2cfc))
* protect common trunk branches in pre-commit hook ([#179](https://github.com/VitruvianSoftware/devx/issues/179)) ([0745f88](https://github.com/VitruvianSoftware/devx/commit/0745f88cc7da62bedbc862c8451df87e3fd5e545))
* wire devx ci run into envvault + add S3/MinIO emulator ([#180](https://github.com/VitruvianSoftware/devx/issues/180)) ([1da5ad5](https://github.com/VitruvianSoftware/devx/commit/1da5ad58644e136665bd6fc13bd4764dd05e4d0d))


### Bug Fixes

* **ship:** split commit message into PR title + body so multi-line -m works ([#189](https://github.com/VitruvianSoftware/devx/issues/189)) ([99acc06](https://github.com/VitruvianSoftware/devx/commit/99acc066dcf39c900d99576a91d2f5f6102ff2ab))

## [0.44.0](https://github.com/VitruvianSoftware/devx/compare/v0.43.1...v0.44.0) (2026-05-12)


### Features

* **ship:** extensible pipeline with multi-stack detection and CI-parity config ([c0e734f](https://github.com/VitruvianSoftware/devx/commit/c0e734fab505f0bb8154773f53199dfbce2c53df))
* **ship:** shift left github CI linting parity to local pre-flight checks ([#175](https://github.com/VitruvianSoftware/devx/issues/175)) ([17ffb77](https://github.com/VitruvianSoftware/devx/commit/17ffb77c1d14ea89514ead4458b212bed47c1186))
* **telemetry:** add CLI error rate, env bootstrap latency, preflight success, and test failure panels ([c20a417](https://github.com/VitruvianSoftware/devx/commit/c20a417eb09aa8933afb9b2b1650bb4701764d50))
* **telemetry:** add service.version to OTel spans and version tracking dashboard panels ([8ce14ff](https://github.com/VitruvianSoftware/devx/commit/8ce14ff470f720ee88d17669ee27c66e9c2b394c))


### Bug Fixes

* **telemetry:** correct go_test exact match queries to regex and add missing devx_action/up_startup panels ([8e8be63](https://github.com/VitruvianSoftware/devx/commit/8e8be63552f701e01e7e5d7f4c726a4f940a1f62))

## [0.43.1](https://github.com/VitruvianSoftware/devx/compare/v0.43.0...v0.43.1) (2026-05-12)


### Bug Fixes

* correct datasource UIDs in Grafana dashboard provisioning ([7d9cb26](https://github.com/VitruvianSoftware/devx/commit/7d9cb26d4592108d405c0970ec5d6268d4e2e50f))
* **telemetry:** correct dashboard datasources and flush metrics on exit ([13802b1](https://github.com/VitruvianSoftware/devx/commit/13802b18d312594f09920337be5ba1f1bee39439))

## [0.43.0](https://github.com/VitruvianSoftware/devx/compare/v0.42.0...v0.43.0) (2026-05-04)


### Features

* **doctor:** rework CLI tool output to feature-area flat list ([#168](https://github.com/VitruvianSoftware/devx/issues/168)) ([18faa8b](https://github.com/VitruvianSoftware/devx/commit/18faa8bcbbc3a826f1f3c9c4c7de4492c887201d))

## [0.42.0](https://github.com/VitruvianSoftware/devx/compare/v0.41.0...v0.42.0) (2026-05-04)


### Features

* add basic entry point execution to main ([28eb400](https://github.com/VitruvianSoftware/devx/commit/28eb400621e4778661a687a40fb1127407622f0e))
* intelligent failure recovery and db ask ([#167](https://github.com/VitruvianSoftware/devx/issues/167)) ([a7bc507](https://github.com/VitruvianSoftware/devx/commit/a7bc507d3015601add55deefc775269fcfb674ad))


### Bug Fixes

* **ai:** dynamically select first available model for ollama launch ([09c5c3a](https://github.com/VitruvianSoftware/devx/commit/09c5c3a383716a4c8b8871e19c830c8202c117bb))

## [0.41.0](https://github.com/VitruvianSoftware/devx/compare/v0.40.0...v0.41.0) (2026-05-04)


### Features

* **agent:** AI-powered commit messages and code review via ollama launch ([9018811](https://github.com/VitruvianSoftware/devx/commit/9018811f63555e32b3e177a7ac4dd0ac6433f53b))
* **agent:** integrate ollama launch --config into devx agent init ([5f261a0](https://github.com/VitruvianSoftware/devx/commit/5f261a08caaf43456538f7ed3d4a7cb72be3bd43))

## [0.40.0](https://github.com/VitruvianSoftware/devx/compare/v0.39.1...v0.40.0) (2026-05-04)


### Features

* **doctor:** add AI Landscape section to detect LLMs, cloud APIs, and coding agents ([5432cfa](https://github.com/VitruvianSoftware/devx/commit/5432cfad1cf6fd4b71484ccb4fa0c9decccae47a))

## [0.39.1](https://github.com/VitruvianSoftware/devx/compare/v0.39.0...v0.39.1) (2026-05-04)


### Bug Fixes

* **agent:** rename internal agent templates target to .agents for antigravity skills ([d95afa4](https://github.com/VitruvianSoftware/devx/commit/d95afa4ad4b91f024903b709aae7a31b08f88557))
* **updater:** support GITHUB_TOKEN to bypass GitHub API rate limits during devx upgrade ([fc3b927](https://github.com/VitruvianSoftware/devx/commit/fc3b92743ad082cc7c71995f344d0c098d3e1858))

## [0.39.0](https://github.com/VitruvianSoftware/devx/compare/v0.38.0...v0.39.0) (2026-05-04)


### Features

* implement AI-driven synthetic data generation (Idea 57) ([#161](https://github.com/VitruvianSoftware/devx/issues/161)) ([2ea31d6](https://github.com/VitruvianSoftware/devx/commit/2ea31d6cee96a80304f0c95b73359b7aac706b5f))

## [0.38.0](https://github.com/VitruvianSoftware/devx/compare/v0.37.0...v0.38.0) (2026-05-03)


### Features

* **state:** implement peer-to-peer state replication (Idea 56) ([#160](https://github.com/VitruvianSoftware/devx/issues/160)) ([01973fb](https://github.com/VitruvianSoftware/devx/commit/01973fb860274eace79a82d8fab6c07b8adb62e3))


### Bug Fixes

* **scaffold:** bump pgx v5.6.0 → v5.9.2 in go-api template ([0c36416](https://github.com/VitruvianSoftware/devx/commit/0c364169c4d55a3b3674f187362364403d19a793))

## [0.37.0](https://github.com/VitruvianSoftware/devx/compare/v0.36.0...v0.37.0) (2026-05-03)


### Features

* implement devx preview for instant PR sandboxing (Idea 55) ([#154](https://github.com/VitruvianSoftware/devx/issues/154)) ([8b3c859](https://github.com/VitruvianSoftware/devx/commit/8b3c8597a23a0bf72f94a0e4065cad16352f1f54))


### Bug Fixes

* **docs:** exclude internal plans and analysis from vitepress build ([#158](https://github.com/VitruvianSoftware/devx/issues/158)) ([487d10e](https://github.com/VitruvianSoftware/devx/commit/487d10e6cbe952351d158d453bc91337a256185b))

## [0.36.0](https://github.com/VitruvianSoftware/devx/compare/v0.35.0...v0.36.0) (2026-05-03)


### Features

* trigger release for cluster refactor ([ed5ede3](https://github.com/VitruvianSoftware/devx/commit/ed5ede35ace7ed7ae3927b0a189e1078ea531c9a))

## [0.35.0](https://github.com/VitruvianSoftware/devx/compare/v0.34.0...v0.35.0) (2026-05-03)


### Features

* implement upward configuration discovery ([eeac698](https://github.com/VitruvianSoftware/devx/commit/eeac6984040d91766490e50b440cf9d9d173c16e))


### Bug Fixes

* ignore os.Chdir error in cleanup to satisfy errcheck linter ([3281ff3](https://github.com/VitruvianSoftware/devx/commit/3281ff3d9a29974e7a30c6544a21c2cc1bdc4948))

## [0.34.0](https://github.com/VitruvianSoftware/devx/compare/v0.33.0...v0.34.0) (2026-05-02)


### Features

* decouple CLI from Podman, support multi-provider virtualization (Lima, Colima, Docker, OrbStack) ([#132](https://github.com/VitruvianSoftware/devx/issues/132)) ([ba1103c](https://github.com/VitruvianSoftware/devx/commit/ba1103cf381f0c0af5d72aacc23fb9ca2d384a11))


### Bug Fixes

* replace remaining deprecated strings.Title in cmd/audit.go ([#135](https://github.com/VitruvianSoftware/devx/issues/135)) ([e68f1bb](https://github.com/VitruvianSoftware/devx/commit/e68f1bb04f0f2963b3f01af9e76f99129ec9d5fb))
* resolve CI failures by skipping missing provider binary tests and addressing string Title deprecation lint error ([#134](https://github.com/VitruvianSoftware/devx/issues/134)) ([b7a2494](https://github.com/VitruvianSoftware/devx/commit/b7a249489f2b8f560821ad96c4c20979fe5451b4))

## [0.33.0](https://github.com/VitruvianSoftware/devx/compare/v0.32.1...v0.33.0) (2026-05-02)


### Features

* merge homelab tool into devx as a subcommand ([#130](https://github.com/VitruvianSoftware/devx/issues/130)) ([93f3524](https://github.com/VitruvianSoftware/devx/commit/93f35244c0e802fdcbe7bcad213e8ff984ab7ef3))

## [0.32.1](https://github.com/VitruvianSoftware/devx/compare/v0.32.0...v0.32.1) (2026-04-13)


### Bug Fixes

* trigger release for homebrew configuration ([8583590](https://github.com/VitruvianSoftware/devx/commit/8583590ebf3a092e436b00d2b32a688778deac46))

## [0.32.0](https://github.com/VitruvianSoftware/devx/compare/v0.31.0...v0.32.0) (2026-04-05)


### Features

* **bridge:** implement full hybrid topology (Idea 46.3) ([d536902](https://github.com/VitruvianSoftware/devx/commit/d536902907bedb4e8efa46d90126841123ec32e6))
* implement Idea 46.1 hybrid edge-to-local bridge via kubectl port-forward ([#127](https://github.com/VitruvianSoftware/devx/issues/127)) ([563ddcb](https://github.com/VitruvianSoftware/devx/commit/563ddcbf6f0b68784447ab3869f2fccf6f691948))
* implement idea 46.2 inbound traffic interception ([c3731b7](https://github.com/VitruvianSoftware/devx/commit/c3731b7a31c19dcb75d8be6bf28fa1bdd59154d5))
* refine devx action ui and telemetry metrics ([424e234](https://github.com/VitruvianSoftware/devx/commit/424e234a9e20f28afa76adf9c61b5f6f3ad10a0f))


### Bug Fixes

* **bridge:** preserve DAG sessions during disconnect and fix SA4010 ([3b0efcf](https://github.com/VitruvianSoftware/devx/commit/3b0efcf4147bf22c23f3e0a852b211bc9eb10747))
* resolve golangci-lint unhandled errors and ineffectual assignment ([#128](https://github.com/VitruvianSoftware/devx/issues/128)) ([2d5f7d4](https://github.com/VitruvianSoftware/devx/commit/2d5f7d4043a0cd85c9c239dcb1c85ca25d6fbd2a))

## [0.31.0](https://github.com/VitruvianSoftware/devx/compare/v0.30.0...v0.31.0) (2026-04-05)


### Features

* **pipeline:** implement lifecycle hooks and custom actions (Idea 45.4) ([43a4660](https://github.com/VitruvianSoftware/devx/commit/43a4660f480237a7e02798ee29b5787dd10d8c19))

## [0.30.0](https://github.com/VitruvianSoftware/devx/compare/v0.29.0...v0.30.0) (2026-04-04)


### Features

* **observability:** Implement granular test telemetry and fix Tempo metrics ([#123](https://github.com/VitruvianSoftware/devx/issues/123)) ([e19bf58](https://github.com/VitruvianSoftware/devx/commit/e19bf5830d8974212d6e2de1c5636584e37ab05d))


### Bug Fixes

* **docs:** resolve Vitepress build failure from absolute paths ([0c521d2](https://github.com/VitruvianSoftware/devx/commit/0c521d2565eb598580ab3ba9b1f18c0b7db6fdc4))
* ineffectual assignment in run.go ([3c5524b](https://github.com/VitruvianSoftware/devx/commit/3c5524b42231578bd6702d35bb712a4b680289d6))

## [0.29.0](https://github.com/VitruvianSoftware/devx/compare/v0.28.0...v0.29.0) (2026-04-04)


### Features

* bridge build telemetry to local OTel observability with auto-provisioned Grafana dashboard (Idea 45.1) ([#122](https://github.com/VitruvianSoftware/devx/issues/122)) ([7413a71](https://github.com/VitruvianSoftware/devx/commit/7413a71ca713775edc49f0c6b20948a9a5ce25a6))
* implement unified multirepo orchestration (Idea 44) ([#117](https://github.com/VitruvianSoftware/devx/issues/117)) ([5ed867f](https://github.com/VitruvianSoftware/devx/commit/5ed867f1d1879399125c8f795de74ff5b35e1716))
* predictive pre-building telemetry + devx stats (Idea 45, Phase 1-2) ([#120](https://github.com/VitruvianSoftware/devx/issues/120)) ([70badb3](https://github.com/VitruvianSoftware/devx/commit/70badb3a4f3f26125aa5ce61cb24c3ba635cd292))


### Bug Fixes

* resolve EnvFile included variables and relocate mergeProfile (Idea 44 follow-up) ([#119](https://github.com/VitruvianSoftware/devx/issues/119)) ([fc3fcce](https://github.com/VitruvianSoftware/devx/commit/fc3fcce74126f74f42d0d9dbd977b7e3dd88765f))
* resolve errcheck lint violations in telemetry tests ([8ca20d3](https://github.com/VitruvianSoftware/devx/commit/8ca20d3abc6ef851da15eea3ca97a5b77606e6ea))
* satisfy errcheck on deferred flock unlock in telemetry metrics ([#121](https://github.com/VitruvianSoftware/devx/issues/121)) ([096b6f1](https://github.com/VitruvianSoftware/devx/commit/096b6f1f99b426428ad439eae39a508638a71b67))

## [0.28.0](https://github.com/VitruvianSoftware/devx/compare/v0.27.3...v0.28.0) (2026-04-03)


### Features

* **cli:** add db seed data runner for automated local data injection ([39c1f5f](https://github.com/VitruvianSoftware/devx/commit/39c1f5fe00b07cc23ed057b22bfabeff60921e6f))
* smart file syncing via mutagen (Idea 43) ([#116](https://github.com/VitruvianSoftware/devx/issues/116)) ([bca5279](https://github.com/VitruvianSoftware/devx/commit/bca5279091e3162d5fa1ae72d263bdc250683ebd))

## [0.27.3](https://github.com/VitruvianSoftware/devx/compare/v0.27.2...v0.27.3) (2026-04-03)


### Bug Fixes

* **ci:** resolve golangci-lint warnings in ci parser and executor ([#113](https://github.com/VitruvianSoftware/devx/issues/113)) ([88e830c](https://github.com/VitruvianSoftware/devx/commit/88e830ce6b05b2d7e6618daf90d9e1b40d26a8d4))

## [0.27.2](https://github.com/VitruvianSoftware/devx/compare/v0.27.1...v0.27.2) (2026-04-03)


### Bug Fixes

* **docs:** escape vue interpolations in markdown ([605dd89](https://github.com/VitruvianSoftware/devx/commit/605dd892b3715a2b61b4caa43b842fa63155f031))
* **docs:** escape vue interpolations in markdown ([766ceb0](https://github.com/VitruvianSoftware/devx/commit/766ceb08b9395613f915e0424a9c2643cfcf0b00))

## [0.27.1](https://github.com/VitruvianSoftware/devx/compare/v0.27.0...v0.27.1) (2026-04-03)


### Bug Fixes

* **devxerr:** add interactive gcloud auth auto-recovery for container tasks ([#109](https://github.com/VitruvianSoftware/devx/issues/109)) ([e8df755](https://github.com/VitruvianSoftware/devx/commit/e8df7554d474addc761ce36328aab0a5bea50e71))

## [0.27.0](https://github.com/VitruvianSoftware/devx/compare/v0.26.0...v0.27.0) (2026-04-03)


### Features

* implement devx ci run — local CI pipeline emulation (Idea 42) ([#107](https://github.com/VitruvianSoftware/devx/issues/107)) ([076e737](https://github.com/VitruvianSoftware/devx/commit/076e737ebb8d14ec59b526918df8d683025cbaf1))

## [0.26.0](https://github.com/VitruvianSoftware/devx/compare/v0.25.0...v0.26.0) (2026-04-03)


### Features

* Devx State Command Hierarchy (Idea 41 & 47) ([da85676](https://github.com/VitruvianSoftware/devx/commit/da85676eb182c2ab202f2ea87f16861c6d07dc7e))
* implement devx agent ship — deterministic agentic pipeline guardrail (Idea 40) ([#105](https://github.com/VitruvianSoftware/devx/issues/105)) ([f9251ba](https://github.com/VitruvianSoftware/devx/commit/f9251badb8b6aae4261c182d5992f888cff82e52))


### Bug Fixes

* apply strict documentation checks to embed template ([#100](https://github.com/VitruvianSoftware/devx/issues/100)) ([3afe719](https://github.com/VitruvianSoftware/devx/commit/3afe719a9db593427dc468fad48eda68cc43b50e))
* downgrade go.mod to 1.24.0 to resolve golangci-lint CI failure ([#104](https://github.com/VitruvianSoftware/devx/issues/104)) ([b54bd26](https://github.com/VitruvianSoftware/devx/commit/b54bd26887e1eeac0ec9edf627c771b925f05689))
* resolve 3 bugs and 4 design issues in devx state implementation ([5fda58e](https://github.com/VitruvianSoftware/devx/commit/5fda58e7ed9f2142753e281c9ae55b3937ecfa0f))

## [0.25.0](https://github.com/VitruvianSoftware/devx/compare/v0.24.0...v0.25.0) (2026-04-03)


### Features

* implement DAG-based service orchestration (Ideas 34/35/36) ([#94](https://github.com/VitruvianSoftware/devx/issues/94)) ([e86d663](https://github.com/VitruvianSoftware/devx/commit/e86d663e486dddf3f011f582097e1a211832439a))
* P1 Polish Pass — Environment Profiles, Secrets Redaction, Visual Map (Ideas 37/38/39) ([#97](https://github.com/VitruvianSoftware/devx/issues/97)) ([4722b7f](https://github.com/VitruvianSoftware/devx/commit/4722b7fd567fd8ee936cb81c148db152f5f259c7))


### Bug Fixes

* handle errcheck lint failures in DAG test ([#96](https://github.com/VitruvianSoftware/devx/issues/96)) ([5f82d00](https://github.com/VitruvianSoftware/devx/commit/5f82d00956397c546ed15b7118d49929605277a4))

## [0.24.0](https://github.com/VitruvianSoftware/devx/compare/v0.23.0...v0.24.0) (2026-04-02)


### Features

* proactive CLI error resolutions ([3b35ab4](https://github.com/VitruvianSoftware/devx/commit/3b35ab4987cf8eebe0d770b46dea12724b0aabaf))
* proactive user-friendly auto-resolution for common CLI errors ([8498c11](https://github.com/VitruvianSoftware/devx/commit/8498c112483ef497580572558af8683b169a2aa0))

## [0.23.0](https://github.com/VitruvianSoftware/devx/compare/v0.22.0...v0.23.0) (2026-04-02)


### Features

* **k8s:** implement zero-config devx k8s local clusters via single-binary k3s ([#91](https://github.com/VitruvianSoftware/devx/issues/91)) ([c68fa35](https://github.com/VitruvianSoftware/devx/commit/c68fa353e19322a1debf67ccd9cf5b47791b8ea5))
* **mock:** implement devx mock for OpenAPI 3rd-party API mocking ([#89](https://github.com/VitruvianSoftware/devx/issues/89)) ([858ce06](https://github.com/VitruvianSoftware/devx/commit/858ce06337da96f7bbb70d3f99354d514f018e7d))
* **testing:** implement devx test ui for ephemeral browser testing isolation ([#85](https://github.com/VitruvianSoftware/devx/issues/85)) ([0b0a88b](https://github.com/VitruvianSoftware/devx/commit/0b0a88b28e747822d99007372da96a5ca73b7e0c))


### Bug Fixes

* **docs:** copy visual proof image to public asset directory to resolve VitePress CI build error ([#86](https://github.com/VitruvianSoftware/devx/issues/86)) ([8bdc59a](https://github.com/VitruvianSoftware/devx/commit/8bdc59a91fe665090884c5f9fa4192406dd99420))
* **lint:** Ignore error returns in test helpers ([19d4a6f](https://github.com/VitruvianSoftware/devx/commit/19d4a6f7282211ebdd424dd1e16992b8594a0de0))
* **mock:** handle Sscanf error return to satisfy errcheck linter ([#90](https://github.com/VitruvianSoftware/devx/issues/90)) ([23dcda9](https://github.com/VitruvianSoftware/devx/commit/23dcda9251821636c185295a854053f74a6d2162))
* **vault:** convert Bitwarden sync to native Go with auto-login and schema provisioning ([#83](https://github.com/VitruvianSoftware/devx/issues/83)) ([27efafe](https://github.com/VitruvianSoftware/devx/commit/27efafef0f6b791229923c418b3551ea4bbf3286))

## [0.22.0](https://github.com/VitruvianSoftware/devx/compare/v0.21.0...v0.22.0) (2026-04-02)


### Features

* **agent:** multi-skill orchestrator + shift-left observability docs ([903f54e](https://github.com/VitruvianSoftware/devx/commit/903f54e89373f8d4ca050d313392cafed853d71b))
* **trace:** shift-left distributed observability via devx trace ([#81](https://github.com/VitruvianSoftware/devx/issues/81)) ([fcad429](https://github.com/VitruvianSoftware/devx/commit/fcad42955f4555239868d8884b6c0d8eb0640abc))

## [0.21.0](https://github.com/VitruvianSoftware/devx/compare/v0.20.1...v0.21.0) (2026-04-01)


### Features

* **ai:** zero-friction local AI bridge and agentic workflow mounts ([#80](https://github.com/VitruvianSoftware/devx/issues/80)) ([c8f1812](https://github.com/VitruvianSoftware/devx/commit/c8f1812c8753fad951ef5c24a985285d1040c63d))
* **scaffold:** new devx scaffold command with 6 built-in templates ([#74](https://github.com/VitruvianSoftware/devx/issues/74)) ([ca2163f](https://github.com/VitruvianSoftware/devx/commit/ca2163f45e265cc22f0e8b70f0193084e1e0712d))


### Bug Fixes

* **scaffold:** make scaffold idempotent by default ([#76](https://github.com/VitruvianSoftware/devx/issues/76)) ([8ef7098](https://github.com/VitruvianSoftware/devx/commit/8ef70985c9c5f90539bdca4e8e2cf7990958106c))
* **scaffold:** resolve go vet warning for redundant newlines in Println ([#78](https://github.com/VitruvianSoftware/devx/issues/78)) ([a50646e](https://github.com/VitruvianSoftware/devx/commit/a50646e3aebf54d0efa977240a1073c025966042))

## [0.20.1](https://github.com/VitruvianSoftware/devx/compare/v0.20.0...v0.20.1) (2026-04-01)


### Bug Fixes

* **ci:** actually merge goreleaser into release-please ([#73](https://github.com/VitruvianSoftware/devx/issues/73)) ([3dd18e8](https://github.com/VitruvianSoftware/devx/commit/3dd18e80f27476332cbdc846e1252955559c182c))
* **ci:** wire goreleaser directly into release-please pipeline ([#71](https://github.com/VitruvianSoftware/devx/issues/71)) ([29006dd](https://github.com/VitruvianSoftware/devx/commit/29006dd2514f8e53cd9ff1b4fc31d96ff8555650))

## [0.20.0](https://github.com/VitruvianSoftware/devx/compare/v0.19.0...v0.20.0) (2026-04-01)


### Features

* **ci:** automate versioning and changelog generation via release-please ([#68](https://github.com/VitruvianSoftware/devx/issues/68)) ([d717ac2](https://github.com/VitruvianSoftware/devx/commit/d717ac290100fdec6f4a06195a298dba3a5382f5))

## [0.19.0] - 2026-04-01

### Added
- **Instant Security Auditing** (`devx audit`)
  - Pre-push vulnerability (Trivy) and secret (Gitleaks) scanning
  - Zero-install architecture runs missing tools automatically via ephemeral read-only Podman/Docker containers
  - One-line git hooks integration (`devx audit install-hooks`)
  - Bypasses `gcloud` credential helper conflicts securely for public images
- **Zero-Friction Production Data Sync** (`devx db pull`)
  - Pulls pre-anonymized databases directly into local containers
  - New parallel binary ingestion mode (`pg_restore -j <N>`) for 5GB+ Postgres databases
  - Standard SQL streaming for MySQL/MongoDB operations
- **AI Agent Tooling & Workflows** (`v0.8.0` - `v0.15.0`)
  - Official agent skills directory (`.agents/skills`) with `--force` upgrade system
  - Predictable exit codes and unified JSON output hooks (`--json`)
  - Global AI override flags (`--dry-run`, `--non-interactive`)
- **Documentation Site**
  - Deployed comprehensive Vitepress documentation site matching the CLI feature set
- **Site Deployment** (`devx sites init`)
  - Automated GitHub Pages and Cloudflare DNS provisioning via interactive wizard
- **Advanced Infrastructure**
  - Devcontainer integration (`devx shell`)
  - Multi-port topology parsing via `devx.yaml`
  - Built-in basic auth for exposed tunnels
  - One-click database provisioning (`devx db spawn`)
  - Vault-backed secret synchronization and `.env` automation
  - Native network simulation for fault injection

### Fixed
- Lint errors (`ineffassign`) and dead links in Vitepress build CI pipelines
- Resolved edge cases connecting to sleeping Podman machines during container executions
- Accidental interception of public container registry pulls by gcloud auth helpers

## [0.2.0] - 2026-03-30

### Added
- **Request Inspector TUI** (`devx tunnel inspect [port]`) — a free, open-source replacement for ngrok's paid web inspector
  - Live reverse proxy captures all HTTP request/response pairs
  - Beautiful terminal UI with scrollable request list and detail view
  - One-key replay to resend captured requests
  - Replay tagging to distinguish original vs replayed traffic
  - Optional Cloudflare tunnel exposure via `--expose` flag
  - Full header and body inspection with syntax-aware display
- CHANGELOG.md
- IDEAS.md roadmap document

### Removed
- IMPROVEMENTS.md (replaced by IDEAS.md)

## [0.1.0] - 2026-03-30

### Added
- Initial open-source release
- Nested CLI hierarchy: `vm`, `tunnel`, `config`, `exec`
- Interactive TUI provisioning with Bubble Tea (`devx vm init`)
- ngrok-like port exposure via Cloudflare tunnels (`devx tunnel expose`)
- Port display in tunnel list output
- Local exposure state store (`~/.config/devx/exposures.json`)
- `devx version` command with build-time version injection
- CI pipeline: golangci-lint, tests, cross-platform build matrix, Butane validation
- Release pipeline: GoReleaser with GitHub releases
- Open-source docs: LICENSE (MIT), CONTRIBUTING, CODE_OF_CONDUCT, SECURITY
- GitHub issue and PR templates
- Branch protection on `main` with required status checks

[0.2.0]: https://github.com/VitruvianSoftware/devx/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/VitruvianSoftware/devx/releases/tag/v0.1.0
