package diagnostic

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"syscall"
)

type Problem struct {
	Code      string `json:"code"`
	Stage     string `json:"stage,omitempty"`
	Detail    string `json:"detail"`
	Retryable bool   `json:"retryable,omitempty"`
}

type Failure struct {
	Code  string
	Stage string
	Cause error
}

func (e *Failure) Error() string { return e.Cause.Error() }
func (e *Failure) Unwrap() error { return e.Cause }

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
	p.Retryable = p.Code == "temporary_io" && p.Stage != "publish"
	return p
}

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
