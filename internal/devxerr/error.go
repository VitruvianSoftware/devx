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

package devxerr

import "fmt"

const (
	// State/Existence Errors
	CodeVMDormant  = 15
	CodeVMNotFound = 16

	// Execution/Environment Errors
	CodeHostPortInUse = 22

	// Auth/Authentication Errors
	CodeNotLoggedIn = 41

	// Bridge Errors (Idea 46.1)
	CodeBridgeKubeconfigNotFound = 60 // kubeconfig file does not exist
	CodeBridgeContextUnreachable = 61 // cluster context exists but API server is unreachable
	CodeBridgeNamespaceNotFound  = 62 // target namespace does not exist
	CodeBridgeServiceNotFound    = 63 // target service does not exist in namespace
	CodeBridgePortForwardFailed  = 64 // kubectl port-forward crashed or timed out

	// Bridge Intercept Errors (Idea 46.2)
	CodeBridgeAgentDeployFailed       = 65 // Agent Job failed to deploy or reach Running state
	CodeBridgeAgentHealthFailed       = 66 // Agent /healthz did not return 200 within timeout
	CodeBridgeSelectorPatchFailed     = 67 // Failed to patch Service selector
	CodeBridgeRBACInsufficient        = 68 // Insufficient RBAC permissions for intercept
	CodeBridgeInterceptActive         = 69 // Another intercept is already active for this service
	CodeBridgeUnsupportedProtocol     = 70 // Target port uses UDP (not supported in 46.2)
	CodeBridgeTunnelFailed            = 71 // Yamux tunnel failed to establish or dropped
	CodeBridgeServiceNotInterceptable = 72 // Service has no selector or is ExternalName type

	// State Replication Errors (Idea 56)
	CodeStateShareNoContainers  = 80 // No running devx containers to share
	CodeStateShareUploadFailed  = 81 // Failed to upload bundle to relay/bucket
	CodeStateAttachInvalidID    = 82 // Malformed share ID
	CodeStateAttachDownloadFail = 83 // Failed to download bundle
	CodeStateAttachDecryptFail  = 84 // Wrong passphrase or corrupted bundle
	CodeStateAttachRestoreFail  = 85 // Checkpoint or snapshot restore failed
)

// DevxError wraps a standard error with a stable machine-readable exit code.
type DevxError struct {
	ExitCode int
	Message  string
	Err      error
}

func (e *DevxError) Error() string {
	if e.Err != nil {
		if e.Message != "" {
			return fmt.Sprintf("%s: %v", e.Message, e.Err)
		}
		return e.Err.Error()
	}
	return e.Message
}

// Unwrap support for errors.Is/As
func (e *DevxError) Unwrap() error {
	return e.Err
}

// New returns a new predictable exit code error.
func New(code int, msg string, err error) *DevxError {
	return &DevxError{
		ExitCode: code,
		Message:  msg,
		Err:      err,
	}
}
