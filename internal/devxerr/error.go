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
