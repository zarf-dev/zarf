package generator

import (
	"encoding/json"
	"os"
	"reflect"

	"github.com/defenseunicorns/zarf/src/pkg/message"
	"github.com/defenseunicorns/zarf/src/pkg/utils"
	"github.com/defenseunicorns/zarf/src/types"
)

type RawComponentObject struct {
	ComponentName string
	Required      bool
	Data          []componentData
}

type componentData struct {
	DataType string
	Data     string
}

func getDataFromFlag(dataOrPath string, componentName string) []byte {
	if json.Valid([]byte(dataOrPath)) {
		return []byte(dataOrPath)
	} else if !utils.InvalidPath(dataOrPath) {
		file, err := os.ReadFile(dataOrPath)
		if err != nil {
			message.Fatalf(err, "Could not read file at %s", dataOrPath)
		}
		if json.Valid(file) {
			return file
		} else {
			message.Fatalf("", "The file at %s was not valid JSON", dataOrPath)
		}
	} else {
		message.Fatalf("", "The \"%s\" component has component data that is not valid JSON or a valid path to a json file", componentName)
	}
	return make([]byte, 0)
}

func hasData(t any) bool {
	return !reflect.ValueOf(t).IsZero()
}

func ValidateAndFormatFlags(args []string) []RawComponentObject {

	// Save just the flags passed to "generate" for future use
	var indexOfPkgName int
	for idx, val := range os.Args {
		if args[0] == val {
			indexOfPkgName = idx
			break
		}
	}
	flagsOnly := os.Args[(indexOfPkgName + 1):]

	// Break the array of flags into explicit component definitions
	var rawComponentStringList [][]string
	var nextComponentNameIndex int
	for {
		// Break the loop when we're finished iterating through the flagsOnly array
		if len(flagsOnly) == 0 {
			break
		}

		// Find the index of the next --component-name occurrence
		for i := 0; i < len(flagsOnly); i++ {
			if flagsOnly[i] == "--component-name" && i != 0 {
				nextComponentNameIndex = i
				break
				// If we didn't find any more --component-name, set the next index to the end of the array
			} else if i == len(flagsOnly)-1 {
				nextComponentNameIndex = len(flagsOnly)
			}
		}

		// Append a single component of flags to our string list
		rawComponentStringList = append(rawComponentStringList, flagsOnly[0:nextComponentNameIndex])
		// Remove the flags we appended to our new array from the old one
		flagsOnly = flagsOnly[nextComponentNameIndex:]
	}

	// Format the array of array of flags by components into an array of RawComponentObject
	var formattedComponents []RawComponentObject
	for _, component := range rawComponentStringList {
		newComponent := new(RawComponentObject)

		// Check if the component is missing a data or data-type
		componentDataTypeCount := 0
		componentDataCount := 0
		componentName := ""
		for idx, flag := range component {
			switch flag {
			case "--component-data-type":
				componentDataTypeCount += 1
			case "--component-data":
				componentDataCount += 1
			case "--component-name":
				componentName = component[idx+1]
			}
		}
		if componentDataCount > componentDataTypeCount {
			message.Fatalf("", "Component \"%s\" is missing a \"component-data-type\"", componentName)
		} else if componentDataCount < componentDataTypeCount {
			message.Fatalf("", "Component \"%s\" is missing a \"component-data\"", componentName)
		}

		// Exploit the required ordering of flags to pull data from string arrays into the
		// new object while also validating that the order is correct
		for idx, flag := range component {
			switch flag {
			case "--component-name":
				newComponent.ComponentName = component[idx+1]
			case "--component-data":
				// Enforce flag ordering
				if component[idx-2] != "--component-data-type" {
					errMsg := "Component data in the \"%s\" component is either missing a " +
						"\"--component-data-type\" or out of order, --component-data-type " +
						"must be the flag before --component-data"
					message.Fatalf("", errMsg, newComponent.ComponentName)
				}
			case "--component-data-type":
				// Enforce flag ordering and assign values to the rawComponent
				if component[idx+2] != "--component-data" {
					errMsg := "Component data in the \"%s\" component has a type of \"%s\" " +
						"but either has no data or is out of order, --component-data-type " +
						"must be the flag before --component-data"
					message.Fatalf("", errMsg, newComponent.ComponentName, component[idx+1])
				} else {
					var currentData = componentData{component[idx+1], component[idx+3]}
					newComponent.Data = append(newComponent.Data, currentData)
				}
			case "--required":
				newComponent.Required = true
			}
		}
		if len(newComponent.Data) == 0 {
			message.Warnf("The %s component has no data, this will result in an empty component being written.", newComponent.ComponentName)
		}
		formattedComponents = append(formattedComponents, *newComponent)
	}
	return formattedComponents
}

func CreateZarfPackage(args []string, rawComponents []RawComponentObject) types.ZarfPackage {
	var newPackage types.ZarfPackage

	newPackage.Kind = "ZarfPackageConfig"
	newPackage.Metadata.Name = args[0]

	for _, rawComponent := range rawComponents {
		var newComponent types.ZarfComponent
		newComponent.Name = rawComponent.ComponentName
		newComponent.Required = rawComponent.Required
		for _, rawComponentData := range rawComponent.Data {
			switch rawComponentData.DataType {
			case "scripts":
				if hasData(newComponent.Actions) {
					message.Fatalf("", "The \"%s\" component declared the \"scripts\" data-type more than once", newComponent.Name)
				}

				jsonBytes := getDataFromFlag(rawComponentData.Data, newComponent.Name)
				if len(jsonBytes) == 0 {
					message.Fatalf("", "Unspecified error with component-data in component \"%s\"", newComponent.Name)
				}

				var scripts types.ZarfComponentActions
				err := json.Unmarshal(jsonBytes, &scripts)
				if err != nil || !hasData(scripts) {
					message.Fatalf(err, "\"%s\" does not have valid JSON for the \"scripts\" data-type", newComponent.Name)
				}

				newComponent.Actions = scripts
			case "files":
				if hasData(newComponent.Files) {
					message.Fatalf("", "The \"%s\" component declared the \"files\" data-type more than once", newComponent.Name)
				}

				jsonBytes := getDataFromFlag(rawComponentData.Data, newComponent.Name)
				if len(jsonBytes) == 0 {
					message.Fatalf("", "Unspecified error with component-data in component \"%s\"", newComponent.Name)
				}

				var files []types.ZarfFile
				err := json.Unmarshal(jsonBytes, &files)
				if err != nil || !hasData(files) {
					message.Fatalf(err, "\"%s\" does not have valid JSON for the \"files\" data-type", newComponent.Name)
				}

				newComponent.Files = files
			case "images":
				if hasData(newComponent.Images) {
					message.Fatalf("", "The \"%s\" component declared the \"images\" data-type more than once", newComponent.Name)
				}

				jsonBytes := getDataFromFlag(rawComponentData.Data, newComponent.Name)
				if len(jsonBytes) == 0 {
					message.Fatalf("", "Unspecified error with component-data in component \"%s\"", newComponent.Name)
				}

				var images []string
				err := json.Unmarshal(jsonBytes, &images)
				if err != nil || !hasData(images) {
					message.Fatalf(err, "\"%s\" does not have valid JSON for the \"images\" data-type", newComponent.Name)
				}

				newComponent.Images = images
			case "repos":
				if hasData(newComponent.Repos) {
					message.Fatalf("", "The \"%s\" component declared the \"repos\" data-type more than once", newComponent.Name)
				}

				jsonBytes := getDataFromFlag(rawComponentData.Data, newComponent.Name)
				if len(jsonBytes) == 0 {
					message.Fatalf("", "Unspecified error with component-data in component \"%s\"", newComponent.Name)
				}

				var repos []string
				err := json.Unmarshal(jsonBytes, &repos)
				if err != nil || !hasData(repos) {
					message.Fatalf(err, "\"%s\" does not have valid JSON for the \"repos\" data-type", newComponent.Name)
				}

				newComponent.Repos = repos
			case "dataInjections":
				if hasData(newComponent.DataInjections) {
					message.Fatalf("", "The \"%s\" component declared the \"dataInjections\" data-type more than once", newComponent.Name)
				}

				jsonBytes := getDataFromFlag(rawComponentData.Data, newComponent.Name)
				if len(jsonBytes) == 0 {
					message.Fatalf("", "Unspecified error with component-data in component \"%s\"", newComponent.Name)
				}

				var dataInjections []types.ZarfDataInjection
				err := json.Unmarshal(jsonBytes, &dataInjections)
				if err != nil || !hasData(dataInjections) {
					message.Fatalf(err, "\"%s\" does not have valid JSON for the \"dataInjections\" data-type", newComponent.Name)
				}

				newComponent.DataInjections = dataInjections
			case "charts":
				if hasData(newComponent.Charts) {
					message.Fatalf("", "The \"%s\" component declared the \"charts\" data-type more than once", newComponent.Name)
				}

				jsonBytes := getDataFromFlag(rawComponentData.Data, newComponent.Name)
				if len(jsonBytes) == 0 {
					message.Fatalf("", "Unspecified error with component-data in component \"%s\"", newComponent.Name)
				}

				var charts []types.ZarfChart
				err := json.Unmarshal(jsonBytes, &charts)
				if err != nil || !hasData(charts) {
					message.Fatalf(err, "\"%s\" does not have valid JSON for the \"charts\" data-type", newComponent.Name)
				}

				newComponent.Charts = charts
			case "manifests":
				if hasData(newComponent.Manifests) {
					message.Fatalf("", "The \"%s\" component declared the \"manifests\" data-type more than once", newComponent.Name)
				}

				jsonBytes := getDataFromFlag(rawComponentData.Data, newComponent.Name)
				if len(jsonBytes) == 0 {
					message.Fatalf("", "Unspecified error with component-data in component \"%s\"", newComponent.Name)
				}

				var manifests []types.ZarfManifest
				err := json.Unmarshal(jsonBytes, &manifests)
				if err != nil || !hasData(manifests) {
					message.Fatalf(err, "\"%s\" does not have valid JSON for the \"manifests\" data-type", newComponent.Name)
				}

				newComponent.Manifests = manifests
			default:
				message.Fatalf("", "The component-data-type \"%s\" was unrecognized in component \"%s\"", rawComponentData.DataType, newComponent.Name)
			}
		}
		newPackage.Components = append(newPackage.Components, newComponent)
	}

	return newPackage
}
