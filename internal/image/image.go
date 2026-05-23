// Copyright (c) 2026 VitruvianSoftware
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

// Package image builds developer service images and loads them into a target
// Kubernetes cluster via an in-cluster container registry, so that
// `runtime: kubernetes` services can run locally-built images. This is devx's
// skaffold-class "build + load" step, run before `kubectl apply`.
//
// Mechanism (validated against the lima k3s dev cluster, 2026-05-23): devx
// deploys a registry as a Deployment + NodePort Service in-cluster; images are
// referenced in manifests as localhost:<NodePort>/<name>:<tag>, which every node
// pulls via its own NodePort with no per-node Docker configuration (Docker's
// 127.0.0.0/8 insecure default covers it). devx pushes from the host to a node's
// reachable address:NodePort over an insecure connection.
package image

// Spec declares an image devx builds and loads before applying manifests
// (runtime: kubernetes, kubernetes.images). The built image is pushed as
// <registry RefPrefix>/<Name>:<Tag> (e.g. localhost:30500/brms-offer:dev) —
// reference exactly that in your manifests, with imagePullPolicy: Always so a
// rebuild of the same tag is re-pulled.
type Spec struct {
	Name       string   // image name (required); becomes <RefPrefix>/<Name>:<Tag>
	Context    string   // build context directory (default ".")
	Dockerfile string   // Dockerfile path, relative to Context (default "Dockerfile")
	Tag        string   // image tag (default "dev")
	Platforms  []string // target platforms, e.g. ["linux/amd64","linux/arm64"]; empty = builder default (host arch)
}

// TagOrDefault returns the configured tag, or "dev" when unset.
func (s Spec) TagOrDefault() string {
	if s.Tag != "" {
		return s.Tag
	}
	return "dev"
}

// ContextOrDefault returns the build context directory, or "." when unset.
func (s Spec) ContextOrDefault() string {
	if s.Context != "" {
		return s.Context
	}
	return "."
}

// DockerfileOrDefault returns the Dockerfile path, or "Dockerfile" when unset.
func (s Spec) DockerfileOrDefault() string {
	if s.Dockerfile != "" {
		return s.Dockerfile
	}
	return "Dockerfile"
}
