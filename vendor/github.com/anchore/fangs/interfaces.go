package fangs

// PostLoader is the interface used to do any sort of processing after `config.Load` has been
// called. This runs after the entire struct has been populated from the configuration files and
// environment variables
type PostLoader interface {
	PostLoad() error
}
