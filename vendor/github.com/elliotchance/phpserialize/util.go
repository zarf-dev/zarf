package phpserialize

// StringifyKeys recursively converts a map into a more sensible map with
// strings as keys.
//
// map[interface{}]interface{} is used as an unmarshalling format because PHP
// serialise() permits keys of associative arrays to be non-string. However, in
// reality this is rarely the case and so strings for keys are much more
// compatible with external code.
func StringifyKeys(m map[interface{}]interface{}) (out map[string]interface{}) {
	out = map[string]interface{}{}

	for k, v := range m {
		switch x := v.(type) {
		case []interface{}:
			newSlice := []interface{}{}
			for _, sliceEntry := range x {
				if subMap, ok := sliceEntry.(map[interface{}]interface{}); ok {
					sliceEntry = StringifyKeys(subMap)
				}
				newSlice = append(newSlice, sliceEntry)
			}
			v = newSlice
		case map[interface{}]interface{}:
			v = StringifyKeys(x)
		}

		out[k.(string)] = v
	}

	return
}
