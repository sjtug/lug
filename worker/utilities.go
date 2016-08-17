package worker

type utility interface {
	preHook() error
	postHook() error
}