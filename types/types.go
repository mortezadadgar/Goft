package types

// TODO: import cycle not allowed
// roomsList_templ import postgres packages
// postgres import roomsList_templ for rendering
type Room struct {
	ID          int
	Name        string
	Description string
}
