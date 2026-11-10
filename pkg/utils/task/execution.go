package task

import "context"

// execution lasts from admission until it settles, which can be after a Remove or a Stop has
// forgotten its registration.
type execution struct {
	reg    *registration
	cancel context.CancelFunc // ends it, its retry waits included
	done   chan struct{}      // closed as it settles
	next   *registration      // queued for its task meanwhile, to queue again as it settles
}
