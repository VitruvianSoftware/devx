#!/usr/bin/env python3
# Copyright (c) 2026 VitruvianSoftware
#
# Permission is hereby granted, free of charge, to any person obtaining a copy
# of this software and associated documentation files (the "Software"), to deal
# in the Software without restriction, including without limitation the rights
# to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
# copies of the Software, and to permit persons to whom the Software is
# furnished to do so, subject to the following conditions:
#
# The above copyright notice and this permission notice shall be included in
# all copies or substantial portions of the Software.
#
# THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
# IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
# FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
# AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
# LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
# OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
# SOFTWARE.

import sys
import os
import subprocess
import re
import base64

local_hostname = subprocess.run("hostname", shell=True, capture_output=True, text=True).stdout.strip().lower()

def run_cmd(host, command):
    # Check if host is local
    is_local = (host.lower() == local_hostname or host.lower() == "james-macbook-pro")
    if is_local:
        cmd = f"export PATH=/opt/homebrew/bin:/usr/local/bin:$PATH; {command}"
        res = subprocess.run(cmd, shell=True, capture_output=True, text=True)
        return res.returncode, res.stdout, res.stderr
    else:
        encoded_cmd = base64.b64encode(command.encode('utf-8')).decode('utf-8')
        ssh_command = f"export PATH=/opt/homebrew/bin:/usr/local/bin:$PATH; echo '{encoded_cmd}' | base64 -d | sh"
        res = subprocess.run(["ssh", "-o", "ConnectTimeout=15", "-o", "StrictHostKeyChecking=accept-new", host, ssh_command], capture_output=True, text=True)
        return res.returncode, res.stdout, res.stderr

def run_python_code(host, python_code, run_as_sudo=False):
    encoded = base64.b64encode(python_code.encode('utf-8')).decode('utf-8')
    sudo_prefix = "sudo " if run_as_sudo else ""
    cmd = f"echo '{encoded}' | base64 -d | {sudo_prefix}python3 -"
    return run_cmd(host, cmd)

def run_cmd_in_vm(host, vm_name, command_inside_vm, interpreter="sh", run_as_sudo=False):
    encoded = base64.b64encode(command_inside_vm.encode('utf-8')).decode('utf-8')
    sudo_prefix = "sudo " if run_as_sudo else ""
    host_command = f"echo '{encoded}' | base64 -d | limactl shell {vm_name} {sudo_prefix}{interpreter}"
    return run_cmd(host, host_command)

def parse_cluster_yaml(path):
    with open(path, 'r') as f:
        content = f.read()
    name_match = re.search(r'name:\s*([^\s#\n]+)', content)
    cluster_name = name_match.group(1) if name_match else 'k8s-node'
    nodes = []
    current_node = None
    for line in content.splitlines():
        line_strip = line.strip()
        if not line_strip or line_strip.startswith('#'):
            continue
        if line_strip.startswith('- host:'):
            if current_node:
                nodes.append(current_node)
            host = line_strip.split(':', 1)[1].strip().strip('"').strip("'")
            current_node = {'host': host, 'role': None}
        elif current_node and line_strip.startswith('role:'):
            role = line_strip.split(':', 1)[1].strip().strip('"').strip("'")
            current_node['role'] = role
    if current_node:
        nodes.append(current_node)
    return cluster_name, nodes

def migrate_node(host, role):
    print(f"\n==================================================")
    print(f"Migrating node: {host} (role: {role})")
    print(f"==================================================")
    
    vm_name = "k8s-node"
    
    # 1. Stop VM
    print(f"[{host}] Stopping VM {vm_name}...")
    ret, out, err = run_cmd(host, f"limactl stop {vm_name}")
    print(f"[{host}] Stop VM completed with exit code {ret}")
    
    # 2. Update ~/.lima/k8s-node/lima.yaml
    print(f"[{host}] Updating ~/.lima/{vm_name}/lima.yaml...")
    python_update_yaml = f"""
import os
yaml_path = os.path.expanduser('~/.lima/{vm_name}/lima.yaml')
if not os.path.exists(yaml_path):
    print("lima.yaml not found at", yaml_path)
    exit(1)

with open(yaml_path, 'r') as f:
    content = f.read()

if 'portForwards:' in content:
    if '/var/run/docker.sock' in content:
        print("Docker socket forwarding already configured.")
        exit(0)
    else:
        lines = content.splitlines()
        for i, line in enumerate(lines):
            if line.strip().startswith('portForwards:'):
                indent = len(line) - len(line.lstrip())
                lines.insert(i + 1, " " * (indent + 2) + "- guestSocket: \\"/var/run/docker.sock\\"")
                lines.insert(i + 2, " " * (indent + 4) + "hostSocket: \\"{{{{.Dir}}}}/sock/docker.sock\\"")
                break
        content = '\\n'.join(lines) + '\\n'
else:
    content = content.rstrip() + '\\nportForwards:\\n  - guestSocket: \\"/var/run/docker.sock\\"\\n    hostSocket: \\"{{{{.Dir}}}}/sock/docker.sock\\"\\n'

with open(yaml_path, 'w') as f:
    f.write(content)
print("Updated lima.yaml successfully.")
"""
    ret, out, err = run_python_code(host, python_update_yaml)
    if ret != 0:
        print(f"[{host}] ERROR updating lima.yaml: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] {out.strip()}")
    
    # 3. Start VM
    print(f"[{host}] Starting VM {vm_name}...")
    ret, out, err = run_cmd(host, f"limactl start {vm_name} --tty=false")
    if ret != 0:
        print(f"[{host}] ERROR starting VM: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] VM started successfully.")
    
    # 4. Install Docker inside VM
    print(f"[{host}] Installing Docker CE inside the VM...")
    install_docker_cmd = (
        "curl -fsSL https://get.docker.com | sh\n"
        "for u in $(awk -F: '$3 >= 1000 || $3 == 501 {print $1}' /etc/passwd); do usermod -aG docker \"$u\" || true; done\n"
        "/usr/bin/systemctl daemon-reload\n"
        "/usr/bin/systemctl enable --now docker"
    )
    ret, out, err = run_cmd_in_vm(host, vm_name, install_docker_cmd, interpreter="sh", run_as_sudo=True)
    if ret != 0:
        print(f"[{host}] ERROR installing Docker CE: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] Docker CE installed and started in VM.")
    
    # 5. Restart VM to apply group memberships and activate port forwarding
    print(f"[{host}] Restarting VM to activate socket forwarding...")
    ret, out, err = run_cmd(host, f"limactl stop {vm_name} && limactl start {vm_name} --tty=false")
    if ret != 0:
        print(f"[{host}] ERROR restarting VM: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] VM restarted and socket forwarding active.")
    
    # 6. Patch K3s systemd unit configuration file
    print(f"[{host}] Patching K3s systemd service inside VM...")
    patch_service_script = r"""import sys
import re
import os

service_file = '/etc/systemd/system/k3s.service'
if not os.path.exists(service_file):
    service_file = '/etc/systemd/system/k3s-agent.service'

if not os.path.exists(service_file):
    print("K3s service file not found.")
    sys.exit(1)

with open(service_file, 'r') as f:
    lines = f.read().splitlines()

wants_idx = -1
after_idx = -1
for i, line in enumerate(lines):
    if line.startswith('Wants='):
        wants_idx = i
    if line.startswith('After='):
        after_idx = i

if wants_idx != -1 and 'docker.service' not in lines[wants_idx]:
    lines[wants_idx] += ' docker.service'
if after_idx != -1 and 'docker.service' not in lines[after_idx]:
    lines[after_idx] += ' docker.service'

exec_start_idx = -1
for i, line in enumerate(lines):
    if line.startswith('ExecStart='):
        exec_start_idx = i
        break

if exec_start_idx != -1:
    curr = exec_start_idx
    last_non_empty = exec_start_idx
    while curr < len(lines):
        if not lines[curr].strip():
            break
        if lines[curr].startswith('[') or (curr != exec_start_idx and re.match(r'^\w+=', lines[curr])):
            break
        last_non_empty = curr
        if not lines[curr].rstrip().endswith('\\'):
            break
        curr += 1
    
    has_docker = False
    for j in range(exec_start_idx, last_non_empty + 1):
        if '--docker' in lines[j]:
            has_docker = True
            break
            
    if not has_docker:
        lines[last_non_empty] = lines[last_non_empty].rstrip()
        if not lines[last_non_empty].endswith('\\'):
            lines[last_non_empty] = lines[last_non_empty] + ' \\'
        lines.insert(last_non_empty + 1, "\t'--docker'")

with open(service_file, 'w') as f:
    f.write('\n'.join(lines) + '\n')
print("Patched K3s service file.")
"""
    ret, out, err = run_cmd_in_vm(host, vm_name, patch_service_script, interpreter="python3", run_as_sudo=True)
    if ret != 0:
        print(f"[{host}] ERROR patching K3s service: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] {out.strip()}")
    
    # 7. Reload systemd and restart K3s
    print(f"[{host}] Reloading systemd and restarting K3s service...")
    svc_name = "k3s" if role == "server" else "k3s-agent"
    restart_k3s_cmd = f"/usr/bin/systemctl daemon-reload && /usr/bin/systemctl restart {svc_name}"
    ret, out, err = run_cmd_in_vm(host, vm_name, restart_k3s_cmd, interpreter="sh", run_as_sudo=True)
    if ret != 0:
        print(f"[{host}] ERROR restarting K3s service: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] K3s service restarted successfully.")
    
    # 8. Verification on the node
    print(f"[{host}] Verifying node status...")
    ret, out, err = run_cmd_in_vm(host, vm_name, "docker ps", interpreter="sh", run_as_sudo=False)
    if ret != 0:
        print(f"[{host}] ERROR verifying docker inside VM: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] Docker is running inside VM.")
    
    sock_path = os.path.expanduser(f"~/.lima/{vm_name}/sock/docker.sock")
    ret, out, err = run_cmd(host, f"export DOCKER_HOST=unix://{sock_path} && docker ps")
    if ret != 0:
        print(f"[{host}] ERROR verifying forwarded DOCKER_HOST from host: {err.strip()}")
        sys.exit(1)
    print(f"[{host}] Forwarded DOCKER_HOST verified successfully.")
    print(f"[{host}] Node migration finished successfully!")

def main():
    config_path = "cluster.yaml"
    if not os.path.exists(config_path):
        print(f"Error: {config_path} not found.")
        sys.exit(1)
        
    cluster_name, nodes = parse_cluster_yaml(config_path)
    print(f"Loaded cluster: {cluster_name}")
    print(f"Found {len(nodes)} nodes in config.")
    
    if not nodes:
        print("Error: No nodes found to migrate.")
        sys.exit(1)
        
    init_node = None
    for node in nodes:
        if node['role'] == 'server':
            init_node = node
            break
            
    agents = [n for n in nodes if n['role'] == 'agent']
    servers = [n for n in nodes if n['role'] == 'server' and n != init_node]
    
    rolling_order = agents + servers
    if init_node:
        rolling_order.append(init_node)
        
    print("Migration rolling order:")
    for idx, node in enumerate(rolling_order):
        print(f"  {idx + 1}. {node['host']} ({node['role']})")
        
    for node in rolling_order:
        migrate_node(node['host'], node['role'])
        
    print("\n🎉 Migration of all nodes completed successfully!")

if __name__ == "__main__":
    main()
