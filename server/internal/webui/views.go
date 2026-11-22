package webui

// BaseView carries the fields every page template uses.
type BaseView struct {
	Title         string
	CSRF          string
	Authenticated bool
	Flash         string
	Error         string
}

type errorView struct {
	BaseView
	Status  int
	Message string
}
