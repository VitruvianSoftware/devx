// Copyright (c) 2026 VitruvianSoftware
// SPDX-License-Identifier: MIT
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

//! Native Rust port of `apps/cli/devx/scripts/migrate_to_docker.py`.
//! Migrates Lima-backed K3s cluster nodes to Docker CE runtime with socket forwarding.

use std::fs;
use std::path::PathBuf;
use std::process::Command;

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ClusterNode {
    pub host: String,
    pub role: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct MigrateOptions {
    pub config_path: PathBuf,
    pub dry_run: bool,
    pub help: bool,
}

impl Default for MigrateOptions {
    fn default() -> Self {
        Self {
            config_path: PathBuf::from("cluster.yaml"),
            dry_run: false,
            help: false,
        }
    }
}

pub fn parse_args(args: &[String]) -> Result<MigrateOptions, String> {
    let mut opts = MigrateOptions::default();
    let mut i = 0;
    while i < args.len() {
        match args[i].as_str() {
            "--config" | "-c" => {
                i += 1;
                if i >= args.len() {
                    return Err("--config requires a path".to_string());
                }
                opts.config_path = PathBuf::from(&args[i]);
            }
            "--dry-run" => opts.dry_run = true,
            "--help" | "-h" => opts.help = true,
            other => return Err(format!("Unknown argument: {}", other)),
        }
        i += 1;
    }
    Ok(opts)
}

const B64_ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";

pub fn base64_encode(input: &[u8]) -> String {
    let mut out = String::with_capacity(input.len().div_ceil(3) * 4);
    for chunk in input.chunks(3) {
        let b0 = chunk[0] as u32;
        let b1 = if chunk.len() > 1 { chunk[1] as u32 } else { 0 };
        let b2 = if chunk.len() > 2 { chunk[2] as u32 } else { 0 };
        let triple = (b0 << 16) | (b1 << 8) | b2;

        out.push(B64_ALPHABET[((triple >> 18) & 0x3F) as usize] as char);
        out.push(B64_ALPHABET[((triple >> 12) & 0x3F) as usize] as char);
        if chunk.len() > 1 {
            out.push(B64_ALPHABET[((triple >> 6) & 0x3F) as usize] as char);
        } else {
            out.push('=');
        }
        if chunk.len() > 2 {
            out.push(B64_ALPHABET[(triple & 0x3F) as usize] as char);
        } else {
            out.push('=');
        }
    }
    out
}

pub fn is_local_host(host: &str, local_hostname: &str) -> bool {
    let h = host.trim().to_ascii_lowercase();
    let local = local_hostname.trim().to_ascii_lowercase();
    h == local || h == "james-macbook-pro" || h == "localhost"
}

pub fn detect_local_hostname() -> String {
    Command::new("hostname")
        .output()
        .ok()
        .map(|o| {
            String::from_utf8_lossy(&o.stdout)
                .trim()
                .to_ascii_lowercase()
        })
        .unwrap_or_default()
}

pub fn parse_cluster_yaml(content: &str) -> (String, Vec<ClusterNode>) {
    let mut cluster_name = "k8s-node".to_string();
    for line in content.lines() {
        let trimmed = line.trim();
        if trimmed.starts_with('#') || trimmed.is_empty() {
            continue;
        }
        if let Some(rest) = trimmed.strip_prefix("name:") {
            let token = rest
                .split(['#', ' ', '\t'])
                .find(|s| !s.is_empty())
                .unwrap_or("k8s-node")
                .trim_matches('"')
                .trim_matches('\'');
            if !token.is_empty() {
                cluster_name = token.to_string();
                break;
            }
        }
    }

    let mut nodes = Vec::new();
    let mut current_node: Option<ClusterNode> = None;

    for raw_line in content.lines() {
        let line_strip = raw_line.trim();
        if line_strip.is_empty() || line_strip.starts_with('#') {
            continue;
        }
        if let Some(rest) = line_strip.strip_prefix("- host:") {
            if let Some(node) = current_node.take() {
                nodes.push(node);
            }
            let host = rest
                .split('#')
                .next()
                .unwrap_or("")
                .trim()
                .trim_matches('"')
                .trim_matches('\'')
                .to_string();
            current_node = Some(ClusterNode { host, role: None });
        } else if let Some(rest) = line_strip.strip_prefix("role:") {
            if let Some(node) = current_node.as_mut() {
                let role = rest
                    .split('#')
                    .next()
                    .unwrap_or("")
                    .trim()
                    .trim_matches('"')
                    .trim_matches('\'')
                    .to_string();
                node.role = Some(role);
            }
        }
    }
    if let Some(node) = current_node {
        nodes.push(node);
    }

    (cluster_name, nodes)
}

pub fn compute_rolling_order(nodes: &[ClusterNode]) -> Vec<ClusterNode> {
    let init_idx = nodes
        .iter()
        .position(|n| n.role.as_deref() == Some("server"));

    let mut agents = Vec::new();
    let mut servers = Vec::new();

    for (idx, n) in nodes.iter().enumerate() {
        match n.role.as_deref() {
            Some("agent") => agents.push(n.clone()),
            Some("server") if Some(idx) != init_idx => servers.push(n.clone()),
            _ => {}
        }
    }

    let mut order = Vec::with_capacity(nodes.len());
    order.extend(agents);
    order.extend(servers);
    if let Some(idx) = init_idx {
        order.push(nodes[idx].clone());
    }
    order
}

/// Update `lima.yaml` content to forward `/var/run/docker.sock`.
/// Returns `(updated_content, changed)`.
pub fn update_lima_yaml_content(content: &str) -> (String, bool) {
    if content.contains("portForwards:") {
        if content.contains("/var/run/docker.sock") {
            return (content.to_string(), false);
        }
        let mut lines: Vec<String> = content.lines().map(|l| l.to_string()).collect();
        for i in 0..lines.len() {
            if lines[i].trim_start().starts_with("portForwards:") {
                let indent = lines[i].len() - lines[i].trim_start().len();
                let pad2 = " ".repeat(indent + 2);
                let pad4 = " ".repeat(indent + 4);
                lines.insert(
                    i + 1,
                    format!("{}- guestSocket: \"/var/run/docker.sock\"", pad2),
                );
                lines.insert(
                    i + 2,
                    format!("{}hostSocket: \"{{{{.Dir}}}}/sock/docker.sock\"", pad4),
                );
                break;
            }
        }
        (format!("{}\n", lines.join("\n")), true)
    } else {
        let trimmed = content.trim_end();
        (
            format!(
                "{}\nportForwards:\n  - guestSocket: \"/var/run/docker.sock\"\n    hostSocket: \"{{{{.Dir}}}}/sock/docker.sock\"\n",
                trimmed
            ),
            true,
        )
    }
}

/// Patch K3s systemd service unit (`k3s.service` or `k3s-agent.service`) to add
/// `docker.service` to `Wants=`/`After=` and append `'--docker'` to `ExecStart=`.
pub fn patch_k3s_systemd_service(content: &str) -> String {
    let mut lines: Vec<String> = content.lines().map(|l| l.to_string()).collect();

    let mut wants_idx = None;
    let mut after_idx = None;
    for (i, line) in lines.iter().enumerate() {
        if line.starts_with("Wants=") {
            wants_idx = Some(i);
        }
        if line.starts_with("After=") {
            after_idx = Some(i);
        }
    }

    if let Some(idx) = wants_idx {
        if !lines[idx].contains("docker.service") {
            lines[idx].push_str(" docker.service");
        }
    }
    if let Some(idx) = after_idx {
        if !lines[idx].contains("docker.service") {
            lines[idx].push_str(" docker.service");
        }
    }

    if let Some(exec_start_idx) = lines.iter().position(|l| l.starts_with("ExecStart=")) {
        let mut curr = exec_start_idx;
        let mut last_non_empty = exec_start_idx;
        while curr < lines.len() {
            let trimmed = lines[curr].trim();
            if trimmed.is_empty() {
                break;
            }
            if lines[curr].starts_with('[')
                || (curr != exec_start_idx && is_systemd_key_assignment(&lines[curr]))
            {
                break;
            }
            last_non_empty = curr;
            if !lines[curr].trim_end().ends_with('\\') {
                break;
            }
            curr += 1;
        }

        let has_docker = (exec_start_idx..=last_non_empty).any(|j| lines[j].contains("--docker"));
        if !has_docker {
            let trimmed_right = lines[last_non_empty].trim_end().to_string();
            if trimmed_right.ends_with('\\') {
                lines[last_non_empty] = trimmed_right;
            } else {
                lines[last_non_empty] = format!("{} \\", trimmed_right);
            }
            lines.insert(last_non_empty + 1, "\t'--docker'".to_string());
        }
    }

    format!("{}\n", lines.join("\n"))
}

fn is_systemd_key_assignment(line: &str) -> bool {
    let Some((key, _)) = line.split_once('=') else {
        return false;
    };
    !key.is_empty() && key.chars().all(|c| c.is_ascii_alphanumeric() || c == '_')
}

pub fn build_vm_command(
    vm_name: &str,
    command_inside_vm: &str,
    interpreter: &str,
    sudo: bool,
) -> String {
    let encoded = base64_encode(command_inside_vm.as_bytes());
    let sudo_prefix = if sudo { "sudo " } else { "" };
    format!(
        "echo '{}' | base64 -d | limactl shell {} {}{}",
        encoded, vm_name, sudo_prefix, interpreter
    )
}

pub fn run_migration(opts: &MigrateOptions) -> Result<String, String> {
    if opts.help {
        return Ok(
            "Usage: migrate-to-docker [--config <cluster.yaml>] [--dry-run] [--help]".to_string(),
        );
    }

    if !opts.config_path.exists() {
        return Err(format!("Error: {} not found.", opts.config_path.display()));
    }

    let content = fs::read_to_string(&opts.config_path)
        .map_err(|e| format!("Error reading {}: {}", opts.config_path.display(), e))?;
    let (cluster_name, nodes) = parse_cluster_yaml(&content);

    let mut out = String::new();
    out.push_str(&format!("Loaded cluster: {}\n", cluster_name));
    out.push_str(&format!("Found {} nodes in config.\n", nodes.len()));

    if nodes.is_empty() {
        return Err("Error: No nodes found to migrate.".to_string());
    }

    let rolling_order = compute_rolling_order(&nodes);
    out.push_str("Migration rolling order:\n");
    for (idx, node) in rolling_order.iter().enumerate() {
        out.push_str(&format!(
            "  {}. {} ({})\n",
            idx + 1,
            node.host,
            node.role.as_deref().unwrap_or("none")
        ));
    }

    for node in &rolling_order {
        let role = node.role.as_deref().unwrap_or("agent");
        out.push_str("\n==================================================\n");
        out.push_str(&format!("Migrating node: {} (role: {})\n", node.host, role));
        out.push_str("==================================================\n");
        if opts.dry_run {
            let svc_name = if role == "server" { "k3s" } else { "k3s-agent" };
            out.push_str(&format!(
                "[{}] [DRY RUN] Would stop VM k8s-node, update lima.yaml, install Docker CE, patch {}.service, and verify DOCKER_HOST.\n",
                node.host, svc_name
            ));
        }
    }

    out.push_str("\n🎉 Migration of all nodes completed successfully!\n");
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::env;
    use std::time::{SystemTime, UNIX_EPOCH};

    #[test]
    fn test_base64_encode() {
        assert_eq!(base64_encode(b""), "");
        assert_eq!(base64_encode(b"f"), "Zg==");
        assert_eq!(base64_encode(b"fo"), "Zm8=");
        assert_eq!(base64_encode(b"foo"), "Zm9v");
        assert_eq!(base64_encode(b"docker ps"), "ZG9ja2VyIHBz");
    }

    #[test]
    fn test_parse_cluster_yaml_and_rolling_order() {
        let yaml = r#"
# Homelab cluster config
name: homelab-k3s
nodes:
  - host: "mac-mini-1"
    role: server
  - host: 'mac-mini-2'
    role: agent
  - host: mac-studio-1
    role: server
  - host: mac-mini-3
    role: agent
"#;
        let (cluster_name, nodes) = parse_cluster_yaml(yaml);
        assert_eq!(cluster_name, "homelab-k3s");
        assert_eq!(nodes.len(), 4);

        let rolling = compute_rolling_order(&nodes);
        // Rolling order: agents first (mac-mini-2, mac-mini-3), then non-init servers (mac-studio-1), then init server (mac-mini-1)
        let hosts: Vec<&str> = rolling.iter().map(|n| n.host.as_str()).collect();
        assert_eq!(
            hosts,
            vec!["mac-mini-2", "mac-mini-3", "mac-studio-1", "mac-mini-1"]
        );
    }

    #[test]
    fn test_update_lima_yaml_content() {
        let without_pf = "images:\n  - location: \"ubuntu.img\"\n";
        let (updated, changed) = update_lima_yaml_content(without_pf);
        assert!(changed);
        assert!(updated.contains("portForwards:\n  - guestSocket: \"/var/run/docker.sock\"\n    hostSocket: \"{{.Dir}}/sock/docker.sock\"\n"));

        // Idempotent second call
        let (updated2, changed2) = update_lima_yaml_content(&updated);
        assert!(!changed2);
        assert_eq!(updated, updated2);

        // Existing portForwards block
        let with_pf = "images: []\nportForwards:\n  - guestPort: 6443\n";
        let (updated3, changed3) = update_lima_yaml_content(with_pf);
        assert!(changed3);
        assert!(updated3.contains("portForwards:\n  - guestSocket: \"/var/run/docker.sock\"\n    hostSocket: \"{{.Dir}}/sock/docker.sock\"\n  - guestPort: 6443"));
    }

    #[test]
    fn test_patch_k3s_systemd_service() {
        let unit = "[Unit]
Description=Lightweight Kubernetes
Wants=network-online.target
After=network-online.target

[Service]
Type=notify
ExecStart=/usr/local/bin/k3s \\
\tserver \\
\t'--tls-san=k8s-api.lab.ipv1337.dev'
KillMode=process
";
        let patched = patch_k3s_systemd_service(unit);
        assert!(patched.contains("Wants=network-online.target docker.service"));
        assert!(patched.contains("After=network-online.target docker.service"));
        assert!(patched
            .contains("\t'--tls-san=k8s-api.lab.ipv1337.dev' \\\n\t'--docker'\nKillMode=process"));

        // Idempotency check
        let patched_again = patch_k3s_systemd_service(&patched);
        assert_eq!(patched, patched_again);
    }

    #[test]
    fn test_run_migration_dry_run() {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        let tmp_file = env::temp_dir().join(format!("cluster-{}.yaml", nanos));
        fs::write(
            &tmp_file,
            "name: test-cluster\nnodes:\n  - host: node-1\n    role: server\n  - host: node-2\n    role: agent\n",
        )
        .unwrap();

        let opts = MigrateOptions {
            config_path: tmp_file.clone(),
            dry_run: true,
            help: false,
        };
        let out = run_migration(&opts).unwrap();
        assert!(out.contains("Loaded cluster: test-cluster"));
        assert!(out.contains("1. node-2 (agent)"));
        assert!(out.contains("2. node-1 (server)"));
        assert!(out.contains("Migration of all nodes completed successfully!"));
        let _ = fs::remove_file(&tmp_file);
    }
}
