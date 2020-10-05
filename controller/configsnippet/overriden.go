package configsnippet

type Overridden interface {
	// Utility for checking if the attribute name has been already declared inside the configuration snippet:
	// this is going to be appended at the end of the section, overwriting the previous declared one.
	Overridden(configSnippet string) error
}
