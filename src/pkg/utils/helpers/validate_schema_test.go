package helpers

import "testing"

func TestReadSchema(t *testing.T) {
	t.Run("basic read schema", func(t *testing.T) {
		want := true

		if got := ReadSchema("example.yaml","example-schema.json"); got != want {
			t.Errorf("AppleSauce() = %v, want %v", got, want)
		}
	})
}


// func TestIsZarfValid(t *testing.T) {
// 	t.Run("first test", func(t *testing.T) {
// 		want := false

// 		if got := ZarfYamlIsValid("/home/austin/code/zarf/zarf1.yaml"); got != want {
// 			t.Errorf("ZarfYamlIsValid = %v, want %v", got, want)
// 		}
// 		// if got := ZarfYamlIsValid("/home/austin/code/zarf/zarf.yaml"); got != want {
// 		// 	t.Errorf("AppleSauce() = %v, want %v", got, want)
// 		// }
// 	})
// }

