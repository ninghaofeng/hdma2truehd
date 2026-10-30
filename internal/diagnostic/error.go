// Package diagnostic carries machine-readable failures and scoped local logs
// shared by the audio backend, CLI and desktop application.
package diagnostic

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"syscall"
)

// Problem separates stable policy/localization codes from raw technical detail.
// Retryable is derived by Describe, not inferred from error message text.
type Problem struct {
	Code      string `json:"code"`
	Stage     string `json:"stage,omitempty"`
	Detail    string `json:"detail"`
	Retryable bool   `json:"retryable,omitempty"`
}

// Failure adds stage or policy context without losing errors.Is/As access to
// Cause. An empty Code lets Describe classify the underlying failure.
type Failure struct {
	Code  string
	Stage string
	Cause error
}

func (e *Failure) Error() string { return e.Cause.Error() }
func (e *Failure) Unwrap() error { return e.Cause }

// Wrap adds the caller's stage without replacing the underlying typed cause.
// Describe can still prioritize explicit policy codes over stage fallbacks.
func Wrap(stage string, err error) error {
	if err == nil {
		return nil
	}
	return &Failure{Stage: stage, Cause: err}
}

func Describe(err error) *Problem {
	if err == nil {
		return nil
	}
	p := &Problem{Code: "unexpected", Detail: err.Error()}
	var stage *Failure
	if errors.As(err, &stage) {
		p.Stage = stage.Stage
	}
	var code interface{ DiagnosticCode() string }
	failureCode := explicitCode(err)
	// Explicit policy/cleanup failures outrank wrapped native or OS causes:
	// a temporary cause must not make unsafe cleanup/publication retryable.
	switch {
	case failureCode != "":
		p.Code = failureCode
	case errors.Is(err, context.Canceled):
		p.Code = "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		p.Code = "deadline"
	case errors.As(err, &code):
		p.Code = code.DiagnosticCode()
	case errors.Is(err, fs.ErrPermission), errors.Is(err, syscall.EROFS):
		p.Code = "permission"
	case errors.Is(err, fs.ErrNotExist):
		p.Code = "not_found"
	case errors.Is(err, fs.ErrExist):
		p.Code = "output_exists"
	case errors.Is(err, syscall.ENOSPC):
		p.Code = "no_space"
	case errors.Is(err, syscall.EAGAIN), errors.Is(err, syscall.EINTR), errors.Is(err, syscall.ETIMEDOUT):
		p.Code = "temporary_io"
	case errors.Is(err, syscall.EIO):
		p.Code = "io"
	default:
		switch p.Stage {
		case "probe":
			p.Code = "probe_failed"
		case "encode":
			p.Code = "encode_failed"
		case "check_output", "verify_source", "verify_output":
			p.Code = "verification_failed"
		case "publish":
			p.Code = "publish_failed"
		case "preflight":
			p.Code = "preflight_failed"
		}
	}
	// Generic EIO is not evidence of a transient failure. Publication is never
	// retried automatically because the final name may already be visible.
	p.Retryable = p.Code == "temporary_io" && p.Stage != "publish"
	return p
}

// explicitCode traverses joined cleanup/original errors as well as ordinary
// wrappers; the outer stage-only Failure must not hide an inner policy code.
func explicitCode(err error) string {
	if failure, ok := err.(*Failure); ok && failure.Code != "" {
		return failure.Code
	}
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		for _, cause := range joined.Unwrap() {
			if code := explicitCode(cause); code != "" {
				return code
			}
		}
	} else if cause := errors.Unwrap(err); cause != nil {
		return explicitCode(cause)
	}
	return ""
}

// Transport preserves structured errors across Wails' string-only error channel.
func Transport(err error) error {
	if err == nil {
		return nil
	}
	data, marshalErr := json.Marshal(Describe(err))
	if marshalErr != nil {
		return errors.Join(err, marshalErr)
	}
	return errors.New(string(data))
}
