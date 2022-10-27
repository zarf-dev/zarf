package packager

import (
	"fmt"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/defenseunicorns/zarf/src/config"
	"github.com/defenseunicorns/zarf/src/pkg/k8s"
	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
	"github.com/pterm/pterm"
	"gopkg.in/yaml.v2"
)

const horizontalRule = "───────────────────────────────────────────────────────────────────────────────────────"

func (p *Package) getValidComponents() []types.ZarfComponent {
	message.Debugf("packager.getValidComponents()")

	var validComponentsList []types.ZarfComponent
	var orderedKeys []string
	var choiceComponents []string

	componentGroups := make(map[string][]types.ZarfComponent)

	// The component list is comma-delimited list
	requestedNames := strings.Split(p.cfg.DeployOpts.Components, ",")

	// Init packages use a different component list
	if p.cfg.IsInitConfig {
		requestedNames = strings.Split(p.cfg.InitOpts.Components, ",")
	}

	// Break up components into choice groups
	for _, component := range p.cfg.Pkg.Components {
		key := component.Group
		// If not a choice group, then use the component name as the key
		if key == "" {
			key = component.Name
		} else {
			// Otherwise, add the component name to the choice group list for later validation
			choiceComponents = p.appendIfNotExists(choiceComponents, component.Name)
		}

		// Preserve component order
		orderedKeys = p.appendIfNotExists(orderedKeys, key)

		// Append the component to the list of components in the group
		componentGroups[key] = append(componentGroups[key], component)
	}

	// Loop through each component group in original order and handle required, requested or user confirmation
	for _, key := range orderedKeys {

		componentGroup := componentGroups[key]

		// Choice groups are handled differently for user confirmation
		userChoicePrompt := len(componentGroup) > 1

		// Loop through the components in the group
		for _, component := range componentGroup {
			// First check if the component is required or requested via CLI flag
			requested := p.isRequiredOrRequested(component, requestedNames)

			// If the user has not requested this component via CLI flag, then prompt them if not a choice group
			if !requested && !userChoicePrompt {
				requested = p.confirmOptionalComponent(component)
			}

			if requested {
				// Mark deployment as appliance mode if this is an init config and the k3s component is enabled
				if component.Name == k8s.DistroIsK3s && p.cfg.IsInitConfig {
					p.cfg.InitOpts.ApplianceMode = true
				}
				// Add the component to the list of valid components
				validComponentsList = append(validComponentsList, component)
				// Ensure that the component is not requested again if in a choice group
				userChoicePrompt = false
				// Exit the inner loop on a match since groups should only have one requested component
				break
			}
		}

		// If the user has requested a choice group, then prompt them
		if userChoicePrompt {
			selectedComponent := p.confirmChoiceGroup(componentGroup)
			validComponentsList = append(validComponentsList, selectedComponent)
		}
	}

	// Ensure all user requested components are valid
	if err := p.validateRequests(validComponentsList, requestedNames, choiceComponents); err != nil {
		message.Fatalf(err, "Invalid component argument, %s", err)
	}

	return validComponentsList
}

// Match on the first requested component that is not in the list of valid components and return the component name
func (p *Package) validateRequests(validComponentsList []types.ZarfComponent, requestedComponentNames, choiceComponents []string) error {
	message.Debugf("packager.validateRequests(%#v, %#v, %#v)", validComponentsList, requestedComponentNames, choiceComponents)

	// Loop through each requested component names
	for _, componentName := range requestedComponentNames {
		found := false
		// Match on the first requested component that is a valid component
		for _, component := range validComponentsList {
			if component.Name == componentName {
				found = true
				break
			}
		}

		// If the requested component was not found, then return an error
		if !found {
			// If the requested component is in a choice group, then warn the user they must choose only one
			for _, component := range choiceComponents {
				if component == componentName {
					return fmt.Errorf("component %s is part of a group of components and only one may be chosen", componentName)
				}
			}
			// Otherwise, return an error a gneral error
			return fmt.Errorf("unable to find component %s", componentName)
		}
	}

	return nil
}

func (p *Package) isRequiredOrRequested(component types.ZarfComponent, requestedComponentNames []string) bool {
	message.Debugf("packager.isRequiredOrRequested(%#v, %#v)", component, requestedComponentNames)

	// If the component is required, then just return true
	if component.Required {
		return true
	} else {
		// Otherwise,check if this is one of the components that has been requested
		if len(requestedComponentNames) > 0 || config.CommonOptions.Confirm {
			for _, requestedComponent := range requestedComponentNames {
				// If the component name matches one of the requested components, then return true
				if strings.ToLower(requestedComponent) == component.Name {
					return true
				}
			}
		}
	}

	// All other cases, return false
	return false
}

// Confirm optional component
func (p *Package) confirmOptionalComponent(component types.ZarfComponent) (confirmComponent bool) {
	message.Debugf("packager.confirmOptionalComponent(%#v)", component)

	// Confirm flag passed, just use defaults
	if config.CommonOptions.Confirm {
		return component.Default
	}

	pterm.Println(horizontalRule)

	displayComponent := component
	displayComponent.Description = ""
	content, _ := yaml.Marshal(displayComponent)
	utils.ColorPrintYAML(string(content))
	if component.Description != "" {
		message.Question(component.Description)
	}

	// Since no requested components were provided, prompt the user
	prompt := &survey.Confirm{
		Message: fmt.Sprintf("Deploy the %s component?", component.Name),
		Default: component.Default,
	}
	if err := survey.AskOne(prompt, &confirmComponent); err != nil {
		message.Fatalf(nil, "Confirm selection canceled: %s", err.Error())
	}
	return confirmComponent
}

func (p *Package) confirmChoiceGroup(componentGroup []types.ZarfComponent) types.ZarfComponent {
	message.Debugf("packager.confirmChoiceGroup(%#v)", componentGroup)

	// Confirm flag passed, just use defaults
	if config.CommonOptions.Confirm {
		var componentNames []string
		for _, component := range componentGroup {
			// If the component is default, then return it
			if component.Default {
				return component
			}
			// Add each component name to the list
			componentNames = append(componentNames, component.Name)
		}
		// If no default component was found, give up
		message.Fatalf(nil, "You must specify at least one component from the group %#v when using the --confirm flag.", componentNames)
	}

	pterm.Println(horizontalRule)

	var chosen int
	var options []string

	for _, component := range componentGroup {
		text := fmt.Sprintf("Name: %s\n  Description: %s\n", component.Name, component.Description)
		options = append(options, text)
	}

	prompt := &survey.Select{
		Message: "Select a component to deploy:",
		Options: options,
	}

	if err := survey.AskOne(prompt, &chosen); err != nil {
		message.Fatalf(nil, "Component selection canceled: %s", err.Error())
	}

	return componentGroup[chosen]
}

func (p *Package) appendIfNotExists(slice []string, item string) []string {
	message.Debugf("packager.appendIfNotExists(%#v, %s)", slice, item)

	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}
