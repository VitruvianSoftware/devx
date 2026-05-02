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

// Package cloud defines supported GCP cloud service emulators and their
// container configurations for one-command local provisioning via devx cloud spawn.
package cloud

import "fmt"

// Emulator holds the container configuration for a cloud service emulator.
type Emulator struct {
	Name         string            // Human-readable name (e.g. "Google Cloud Storage")
	Service      string            // Short key (e.g. "gcs")
	Image        string            // Container image
	DefaultPort  int               // Host-side port to bind
	InternalPort int               // Port inside the container
	Env          map[string]string // Default container environment variables
	ReadyLog     string            // Log line indicating the emulator is ready
	EnvVars      map[string]string // Host-side env vars to inject (value is a printf template receiving host:port)
	// TODO: Add S3 emulator (MinIO) when S3 support is needed
}

// Registry of supported GCP cloud emulators.
var Registry = map[string]Emulator{
	"gcs": {
		Name:         "Google Cloud Storage",
		Service:      "gcs",
		Image:        "fsouza/fake-gcs-server:latest",
		DefaultPort:  4443,
		InternalPort: 4443,
		Env:          map[string]string{},
		ReadyLog:     "server started",
		// The GCP client libraries respect STORAGE_EMULATOR_HOST automatically.
		EnvVars: map[string]string{
			"STORAGE_EMULATOR_HOST": "http://%s",
		},
	},
	"pubsub": {
		Name:         "Google Cloud Pub/Sub",
		Service:      "pubsub",
		Image:        "gcr.io/google.com/cloudsdktool/cloud-sdk:latest",
		DefaultPort:  8085,
		InternalPort: 8085,
		Env:          map[string]string{},
		ReadyLog:     "Server started",
		// The Pub/Sub client library uses PUBSUB_EMULATOR_HOST.
		EnvVars: map[string]string{
			"PUBSUB_EMULATOR_HOST": "%s",
		},
	},
	"firestore": {
		Name:         "Google Cloud Firestore",
		Service:      "firestore",
		Image:        "gcr.io/google.com/cloudsdktool/cloud-sdk:latest",
		DefaultPort:  8080,
		InternalPort: 8080,
		Env:          map[string]string{},
		ReadyLog:     "Dev App Server is now running",
		// Firestore client libraries respect FIRESTORE_EMULATOR_HOST.
		EnvVars: map[string]string{
			"FIRESTORE_EMULATOR_HOST": "%s",
		},
	},
}

// SupportedServices returns a sorted list of emulator service names.
func SupportedServices() []string {
	return []string{"gcs", "pubsub", "firestore"}
}

// EnvVarValues returns the resolved env var map for a given host:port string.
func (e Emulator) EnvVarValues(hostPort string) map[string]string {
	out := make(map[string]string, len(e.EnvVars))
	for k, tmpl := range e.EnvVars {
		out[k] = fmt.Sprintf(tmpl, hostPort)
	}
	return out
}
