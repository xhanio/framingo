package errors

import (
	"errors"
	"testing"
)

func TestCombine(t *testing.T) {
	tests := []struct {
		name string
		errs []error
		want string
	}{
		{
			name: "no errors",
			errs: []error{},
			want: "",
		},
		{
			name: "single error",
			errs: []error{Newf("error1")},
			want: "error1",
		},
		{
			name: "multiple errors",
			errs: []error{Newf("error1"), Newf("error2")},
			want: "error1; error2",
		},
		{
			name: "with nil errors",
			errs: []error{Newf("error1"), nil, Newf("error2")},
			want: "error1; error2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Combine(tt.errs...)
			if tt.want == "" {
				if got != nil {
					t.Errorf("Combine() = %v, want nil", got)
				}
			} else {
				if got == nil || got.Error() != tt.want {
					t.Errorf("Combine() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want string
	}{
		{
			name: "empty error",
			opts: []Option{},
			want: "",
		},
		{
			name: "error with message",
			opts: []Option{WithMessage("test error")},
			want: "test error",
		},
		{
			name: "error with code",
			opts: []Option{WithCode("TEST001", map[string]string{"type": "test"})},
			want: "",
		},
		{
			name: "error with message and code",
			opts: []Option{
				WithMessage("test error"),
				WithCode("TEST001", map[string]string{"type": "test"}),
			},
			want: "test error",
		},
		{
			name: "error with category",
			opts: []Option{WithCategory(BadRequest)},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := New(tt.opts...)
			if err.Error() != tt.want {
				t.Errorf("New() = %v, want %v", err.Error(), tt.want)
			}

			// verify it has stack trace
			if eb, ok := err.(*base); ok {
				if eb.stack == nil {
					t.Errorf("New() error should have stack trace")
				}
			}
		})
	}
}

func TestNewf(t *testing.T) {
	tests := []struct {
		name   string
		format string
		args   []any
		want   string
	}{
		{
			name:   "simple message",
			format: "test error",
			args:   []any{},
			want:   "test error",
		},
		{
			name:   "formatted message",
			format: "error: %s %d",
			args:   []any{"test", 123},
			want:   "error: test 123",
		},
		{
			name:   "empty format",
			format: "",
			args:   []any{},
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Newf(tt.format, tt.args...)
			if err.Error() != tt.want {
				t.Errorf("Newf() = %v, want %v", err.Error(), tt.want)
			}

			// verify it has stack trace
			if eb, ok := err.(*base); ok {
				if eb.stack == nil {
					t.Errorf("Newf() error should have stack trace")
				}
			}
		})
	}
}

func TestWrap(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		opts    []Option
		want    string
		wantNil bool
	}{
		{
			name:    "wrap nil error",
			err:     nil,
			opts:    []Option{},
			wantNil: true,
		},
		{
			name: "wrap standard error",
			err:  errors.New("original error"),
			opts: []Option{WithMessage("wrapped")},
			want: "wrapped: original error",
		},
		{
			name: "wrap without options",
			err:  errors.New("original error"),
			opts: []Option{},
			want: "original error",
		},
		{
			name: "wrap base without options returns same error",
			err:  New(WithMessage("base error")),
			opts: []Option{},
			want: "base error",
		},
		{
			name: "wrap base with options",
			err:  New(WithMessage("base error")),
			opts: []Option{WithMessage("wrapped")},
			want: "wrapped: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrap(tt.err, tt.opts...)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Wrap() = %v, want nil", got)
				}
				return
			}

			if got.Error() != tt.want {
				t.Errorf("Wrap() = %v, want %v", got.Error(), tt.want)
			}

			// verify cause is set correctly
			if eb, ok := got.(*base); ok {
				// when wrapping base without options, it returns the same error, so no cause
				if tt.name != "wrap base without options returns same error" && eb.cause != tt.err {
					t.Errorf("Wrap() cause = %v, want %v", eb.cause, tt.err)
				}

				// verify stack trace behavior
				_, isErrBase := tt.err.(*base)
				if !isErrBase && eb.stack == nil {
					t.Errorf("Wrap() should have stack trace for non-base errors")
				}
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		format  string
		args    []any
		want    string
		wantNil bool
	}{
		{
			name:    "wrap nil error",
			err:     nil,
			format:  "wrapped",
			args:    []any{},
			wantNil: true,
		},
		{
			name:   "wrap standard error",
			err:    errors.New("original error"),
			format: "wrapped: %s",
			args:   []any{"test"},
			want:   "wrapped: test: original error",
		},
		{
			name:   "wrap with empty format",
			err:    errors.New("original error"),
			format: "",
			args:   []any{},
			want:   "original error",
		},
		{
			name:   "wrap base with empty format returns same error",
			err:    New(WithMessage("base error")),
			format: "",
			args:   []any{},
			want:   "base error",
		},
		{
			name:   "wrap base with format",
			err:    New(WithMessage("base error")),
			format: "wrapped: %d",
			args:   []any{123},
			want:   "wrapped: 123: base error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Wrapf(tt.err, tt.format, tt.args...)
			if tt.wantNil {
				if got != nil {
					t.Errorf("Wrapf() = %v, want nil", got)
				}
				return
			}

			if got.Error() != tt.want {
				t.Errorf("Wrapf() = %v, want %v", got.Error(), tt.want)
			}

			// verify cause is set correctly
			if eb, ok := got.(*base); ok {
				// when wrapping base with empty format, it returns the same error, so no cause
				if tt.name != "wrap base with empty format returns same error" && eb.cause != tt.err {
					t.Errorf("Wrapf() cause = %v, want %v", eb.cause, tt.err)
				}
			}
		})
	}
}

func TestHas(t *testing.T) {
	cause1 := errors.New("cause1")
	cause2 := errors.New("cause2")
	baseErr := New(WithMessage("base error"))

	tests := []struct {
		name   string
		err    error
		target error
		want   bool
	}{
		{
			name:   "non-Error interface returns false",
			err:    errors.New("standard error"),
			target: cause1,
			want:   false,
		},
		{
			name:   "Error interface with matching cause",
			err:    Wrap(cause1, WithMessage("wrapper")),
			target: cause1,
			want:   true,
		},
		{
			name:   "Error interface without matching cause",
			err:    Wrap(cause1, WithMessage("wrapper")),
			target: cause2,
			want:   false,
		},
		{
			name:   "Error interface checking self",
			err:    baseErr,
			target: baseErr,
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Has(tt.err, tt.target)
			if got != tt.want {
				t.Errorf("Has() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIs(t *testing.T) {
	tests := []struct {
		name string
		err  error
		cat  Category
		want bool
	}{
		{
			name: "non-Error interface returns false",
			err:  errors.New("standard error"),
			cat:  BadRequest,
			want: false,
		},
		{
			name: "Error interface with matching category",
			err:  New(WithCategory(BadRequest)),
			cat:  BadRequest,
			want: true,
		},
		{
			name: "Error interface with different category",
			err:  New(WithCategory(BadRequest)),
			cat:  NotFound,
			want: false,
		},
		{
			name: "Error interface with default category",
			err:  New(WithMessage("no category")),
			cat:  Internal,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Is(tt.err, tt.cat)
			if got != tt.want {
				t.Errorf("Is() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil error",
			err:  nil,
			want: "",
		},
		{
			name: "standard error",
			err:  errors.New("standard error"),
			want: "standard error",
		},
		{
			name: "custom error",
			err:  New(WithMessage("custom error")),
			want: "custom error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Message(tt.err)
			if got != tt.want {
				t.Errorf("Message() = %v, want %v", got, tt.want)
			}
		})
	}
}
