package task

func IsDone(state State) bool {
	return state == StateSucceeded || state == StateFailed || state == StateCanceled
}

func IsPending(state State) bool {
	return state == StateRunning || state == StateCanceling
}
