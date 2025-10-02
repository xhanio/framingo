package errors

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/labels"
)

func TestErrBaseApply(t *testing.T) {
	tests := []struct {
		name string
		opts []Option
		want func(*base) bool
	}{
		{
			name: "apply WithCode option",
			opts: []Option{WithCode("TEST001", map[string]string{"key": "value"})},
			want: func(eb *base) bool {
				return eb.code == "TEST001" && eb.details["key"] == "value"
			},
		},
		{
			name: "apply WithMessage option",
			opts: []Option{WithMessage("test message: %s", "hello")},
			want: func(eb *base) bool {
				return eb.message == "test message: hello"
			},
		},
		{
			name: "apply WithCategory option",
			opts: []Option{WithCategory(Internal)},
			want: func(eb *base) bool {
				return eb.category == Internal
			},
		},
		{
			name: "apply multiple options",
			opts: []Option{
				WithCode("TEST002", map[string]string{"type": "validation"}),
				WithMessage("validation failed"),
				WithCategory(BadRequest),
			},
			want: func(eb *base) bool {
				return eb.code == "TEST002" &&
					eb.message == "validation failed" &&
					eb.category == BadRequest &&
					eb.details["type"] == "validation"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := &base{}
			result := eb.apply(tt.opts...)
			if !tt.want(result) {
				t.Errorf("apply() failed validation for test %s", tt.name)
			}
		})
	}
}

func TestErrBaseRoot(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *base
		want  func(*base, *base) bool
	}{
		{
			name: "single error without cause",
			setup: func() *base {
				return &base{message: "root error"}
			},
			want: func(eb, rootBase *base) bool {
				return rootBase == eb
			},
		},
		{
			name: "error chain with base causes",
			setup: func() *base {
				root := &base{message: "root error"}
				middle := &base{message: "middle error", cause: root}
				top := &base{message: "top error", cause: middle}
				return top
			},
			want: func(eb, rb *base) bool {
				return rb.message == "root error" && rb.cause == nil
			},
		},
		{
			name: "error chain ending with non-base cause",
			setup: func() *base {
				stdErr := fmt.Errorf("standard error")
				wrapper := &base{message: "wrapper", cause: stdErr}
				top := &base{message: "top error", cause: wrapper}
				return top
			},
			want: func(eb, rb *base) bool {
				return rb.message == "wrapper" && rb.cause.Error() == "standard error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			rb := eb.rootBase()
			if !tt.want(eb, rb) {
				t.Errorf("rootBase() failed validation for test %s", tt.name)
			}
		})
	}
}

func TestErrBaseMessage(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *base
		want  string
	}{
		{
			name: "error with message",
			setup: func() *base {
				return &base{message: "test message"}
			},
			want: "test message",
		},
		{
			name: "error without message, with base cause",
			setup: func() *base {
				cause := &base{message: "cause message"}
				return &base{cause: cause}
			},
			want: "cause message",
		},
		{
			name: "error without message, with standard error cause",
			setup: func() *base {
				stdErr := fmt.Errorf("standard error")
				return &base{cause: stdErr}
			},
			want: "standard error",
		},
		{
			name: "chain with empty messages until cause",
			setup: func() *base {
				stdErr := fmt.Errorf("final error")
				middle := &base{cause: stdErr}
				return &base{cause: middle}
			},
			want: "final error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			got := eb.Message()
			if got != tt.want {
				t.Errorf("Message() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrBaseError(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *base
		want  string
	}{
		{
			name: "single error with message",
			setup: func() *base {
				return &base{message: "test error"}
			},
			want: "test error",
		},
		{
			name: "error chain with messages",
			setup: func() *base {
				cause := &base{message: "root cause"}
				middle := &base{message: "middle error", cause: cause}
				return &base{message: "top error", cause: middle}
			},
			want: "top error: middle error: root cause",
		},
		{
			name: "error chain with standard error cause",
			setup: func() *base {
				stdErr := fmt.Errorf("standard error")
				return &base{message: "wrapper", cause: stdErr}
			},
			want: "wrapper: standard error",
		},
		{
			name: "error chain with empty messages",
			setup: func() *base {
				cause := &base{message: "only message"}
				middle := &base{cause: cause}
				return &base{cause: middle}
			},
			want: "only message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			got := eb.Error()
			if got != tt.want {
				t.Errorf("Error() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrBaseCode(t *testing.T) {
	tests := []struct {
		name        string
		setup       func() *base
		wantCode    string
		wantDetails labels.Set
	}{
		{
			name: "error with code and details",
			setup: func() *base {
				return &base{
					code:    "TEST001",
					details: labels.Set{"type": "validation", "field": "email"},
				}
			},
			wantCode:    "TEST001",
			wantDetails: labels.Set{"type": "validation", "field": "email"},
		},
		{
			name: "error without code",
			setup: func() *base {
				return &base{message: "no code"}
			},
			wantCode:    "",
			wantDetails: nil,
		},
		{
			name: "error chain with code in cause",
			setup: func() *base {
				cause := &base{
					code:    "CAUSE001",
					details: labels.Set{"source": "database"},
				}
				return &base{message: "wrapper", cause: cause}
			},
			wantCode:    "CAUSE001",
			wantDetails: labels.Set{"source": "database"},
		},
		{
			name: "error chain with code in top level",
			setup: func() *base {
				cause := &base{message: "cause without code"}
				return &base{
					code:    "TOP001",
					details: labels.Set{"level": "top"},
					cause:   cause,
				}
			},
			wantCode:    "TOP001",
			wantDetails: labels.Set{"level": "top"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			gotCode, gotDetails := eb.Code()
			if gotCode != tt.wantCode {
				t.Errorf("Code() code = %v, want %v", gotCode, tt.wantCode)
			}
			if !labels.Equals(gotDetails, tt.wantDetails) {
				t.Errorf("Code() details = %v, want %v", gotDetails, tt.wantDetails)
			}
		})
	}
}

func TestErrBaseCategory(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *base
		want  Category
	}{
		{
			name: "error with category",
			setup: func() *base {
				return &base{category: BadRequest}
			},
			want: BadRequest,
		},
		{
			name: "error without category defaults to Internal",
			setup: func() *base {
				return &base{message: "no category"}
			},
			want: Internal,
		},
		{
			name: "error chain with category in cause",
			setup: func() *base {
				cause := &base{category: NotFound}
				return &base{message: "wrapper", cause: cause}
			},
			want: NotFound,
		},
		{
			name: "error chain with errCategory cause",
			setup: func() *base {
				categoryErr := newCategory("TestCategory", 418)
				return &base{message: "wrapper", cause: categoryErr}
			},
			want: func() Category {
				return newCategory("TestCategory", 418)
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			got := eb.Category()
			if got.Error() != tt.want.Error() || got.StatusCode() != tt.want.StatusCode() {
				t.Errorf("Category() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrBaseCause(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *base
		want  error
	}{
		{
			name: "error without cause",
			setup: func() *base {
				return &base{message: "no cause"}
			},
			want: nil,
		},
		{
			name: "error with base cause",
			setup: func() *base {
				cause := &base{message: "cause error"}
				return &base{message: "wrapper", cause: cause}
			},
			want: &base{message: "cause error"},
		},
		{
			name: "error with standard error cause",
			setup: func() *base {
				stdErr := fmt.Errorf("standard error")
				return &base{message: "wrapper", cause: stdErr}
			},
			want: fmt.Errorf("standard error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			got := eb.Cause()
			if tt.want == nil {
				if got != nil {
					t.Errorf("Cause() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("Cause() = nil, want non-nil")
				} else if got.Error() != tt.want.Error() {
					t.Errorf("Cause() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}

func TestErrBaseStackTrace(t *testing.T) {
	tests := []struct {
		name    string
		setup   func() *base
		wantNil bool
	}{
		{
			name: "error without stack",
			setup: func() *base {
				return &base{message: "no stack"}
			},
			wantNil: true,
		},
		{
			name: "error with stack",
			setup: func() *base {
				return &base{
					message: "with stack",
					stack:   callers(),
				}
			},
			wantNil: false,
		},
		{
			name: "error chain with stack in root",
			setup: func() *base {
				root := &base{
					message: "root with stack",
					stack:   callers(),
				}
				return &base{message: "wrapper", cause: root}
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			got := eb.StackTrace()
			if tt.wantNil && got != nil {
				t.Errorf("StackTrace() = %v, want nil", got)
			}
			if !tt.wantNil && got == nil {
				t.Errorf("StackTrace() = nil, want non-nil")
			}
		})
	}
}

func TestErrBaseFormat(t *testing.T) {
	eb := &base{
		message: "test error",
		code:    "TEST001",
		details: labels.Set{"type": "test"},
		stack:   callers(),
	}

	tests := []struct {
		name string
		verb rune
		want func(string) bool
	}{
		{
			name: "format with %m verb",
			verb: 'm',
			want: func(s string) bool {
				return s == "test error"
			},
		},
		{
			name: "format with %s verb",
			verb: 's',
			want: func(s string) bool {
				return s == "test error"
			},
		},
		{
			name: "format with %v verb",
			verb: 'v',
			want: func(s string) bool {
				return strings.Contains(s, "test error") && strings.Contains(s, "TEST001")
			},
		},
		{
			name: "format with unknown verb",
			verb: 'x',
			want: func(s string) bool {
				return strings.Contains(s, "!%x(test error)")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := fmt.Sprintf("%"+string(tt.verb), eb)
			if !tt.want(result) {
				t.Errorf("Format() with verb %c failed validation, got: %s", tt.verb, result)
			}
		})
	}
}

func TestErrBaseHas(t *testing.T) {
	cause1 := &base{message: "cause1"}
	cause2 := &base{message: "cause2"}
	stdErr := fmt.Errorf("standard error")

	tests := []struct {
		name   string
		setup  func() *base
		target error
		want   bool
	}{
		{
			name: "has direct cause",
			setup: func() *base {
				return &base{message: "wrapper", cause: cause1}
			},
			target: cause1,
			want:   true,
		},
		{
			name: "has nested cause",
			setup: func() *base {
				middle := &base{message: "middle", cause: cause1}
				return &base{message: "top", cause: middle}
			},
			target: cause1,
			want:   true,
		},
		{
			name: "has standard error cause",
			setup: func() *base {
				return &base{message: "wrapper", cause: stdErr}
			},
			target: stdErr,
			want:   true,
		},
		{
			name: "does not have unrelated error",
			setup: func() *base {
				return &base{message: "wrapper", cause: cause1}
			},
			target: cause2,
			want:   false,
		},
		{
			name: "has self",
			setup: func() *base {
				eb := &base{message: "self"}
				return eb
			},
			target: nil, // will be set to the error itself in test
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			target := tt.target
			if target == nil && tt.name == "has self" {
				target = eb
			}
			got := eb.Has(target)
			if got != tt.want {
				t.Errorf("Has() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrBaseRootCause(t *testing.T) {
	tests := []struct {
		name  string
		setup func() *base
		want  func(error) bool
	}{
		{
			name: "single error without cause",
			setup: func() *base {
				return &base{message: "single error"}
			},
			want: func(err error) bool {
				return err.Error() == "single error"
			},
		},
		{
			name: "error chain ending with standard error",
			setup: func() *base {
				stdErr := fmt.Errorf("root cause")
				wrapper := &base{message: "wrapper", cause: stdErr}
				return &base{message: "top", cause: wrapper}
			},
			want: func(err error) bool {
				return err.Error() == "root cause"
			},
		},
		{
			name: "error chain ending with base",
			setup: func() *base {
				root := &base{message: "root error"}
				middle := &base{message: "middle", cause: root}
				return &base{message: "top", cause: middle}
			},
			want: func(err error) bool {
				return err.Error() == "root error"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			got := eb.RootCause()
			if !tt.want(got) {
				t.Errorf("RootCause() failed validation for test %s, got: %v", tt.name, got)
			}
		})
	}
}

func TestErrBaseChain(t *testing.T) {
	tests := []struct {
		name         string
		setup        func() *base
		wantLen      int
		wantMessages []string
	}{
		{
			name: "single error without cause",
			setup: func() *base {
				return &base{message: "single"}
			},
			wantLen:      1,
			wantMessages: []string{"single"},
		},
		{
			name: "error chain with base causes",
			setup: func() *base {
				root := &base{message: "root"}
				middle := &base{message: "middle", cause: root}
				return &base{message: "top", cause: middle}
			},
			wantLen:      3,
			wantMessages: []string{"top: middle: root", "middle: root", "root"},
		},
		{
			name: "error chain ending with standard error",
			setup: func() *base {
				stdErr := fmt.Errorf("standard error")
				wrapper := &base{message: "wrapper", cause: stdErr}
				return &base{message: "top", cause: wrapper}
			},
			wantLen:      2,
			wantMessages: []string{"top: wrapper: standard error", "wrapper: standard error"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eb := tt.setup()
			got := eb.Chain()
			if len(got) != tt.wantLen {
				t.Errorf("Chain() length = %v, want %v", len(got), tt.wantLen)
			}
			for i, wantMsg := range tt.wantMessages {
				if i >= len(got) {
					t.Errorf("Chain() missing error at index %d", i)
					continue
				}
				if got[i].Error() != wantMsg {
					t.Errorf("Chain()[%d].Error() = %v, want %v", i, got[i].Error(), wantMsg)
				}
			}
		})
	}
}
