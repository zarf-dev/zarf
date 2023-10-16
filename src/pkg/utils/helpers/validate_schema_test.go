package helpers

import "testing"

func TestReadSchema(t *testing.T) {
	t.Run("basic read schema", func(t *testing.T) {
		want := true

		if got := ValidateZarfSchema("/home/austin/code/zarf/zarf.yaml", "../../../../zarf.schema.json"); got != want {
			t.Errorf("AppleSauce() = %v, want %v", got, want)
		}
	})
}
