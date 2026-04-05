package model

import "fmt"

type ErrorCode string

const (
	ErrorConfig   ErrorCode = "E_CONFIG"
	ErrorSpawn    ErrorCode = "E_SPAWN"
	ErrorPromptIO ErrorCode = "E_PROMPT_IO"
	ErrorTimeout  ErrorCode = "E_TIMEOUT"
	ErrorExtract  ErrorCode = "E_EXTRACT"
	ErrorStore    ErrorCode = "E_STORE"
	ErrorRuntime  ErrorCode = "E_RUNTIME"
	ErrorProvider ErrorCode = "E_PROVIDER"
	ErrorInput    ErrorCode = "E_INPUT"
	ErrorNotFound ErrorCode = "E_NOT_FOUND"
	ErrorInternal ErrorCode = "E_INTERNAL"
)

type RunError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func (e *RunError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
