package types

// Generator is the interface for tools that generate service configurations from a user dialog.
type Generator interface {
	// Generate creates a configuration for a service, usually through a dialog
	// with the user.
	//
	// Parameters:
	//   - service: the service to configure. Some generators ignore it.
	//   - props: settings from the user by name. Their meaning depends on the
	//     generator.
	//   - args: extra arguments. Their meaning depends on the generator.
	//
	// Returns:
	//   - ServiceConfig: the generated configuration.
	//   - error: the generation failure, or nil on success.
	Generate(service Service, props map[string]string, args []string) (ServiceConfig, error)
}
