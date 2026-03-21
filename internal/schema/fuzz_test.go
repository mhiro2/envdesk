package schema

import "testing"

func FuzzValidate(f *testing.F) {
	f.Add([]byte("keys:\n  APP_ENV:\n    required: true\n    type: string\n"))
	f.Add([]byte("keys:\n  PORT:\n    required: true\n    type: int\n"))
	f.Add([]byte("keys:\n  DEBUG:\n    type: bool\n"))
	f.Add([]byte("keys:\n  MODE:\n    type: enum\n    values: [dev, prod]\n"))
	f.Add([]byte("keys:\n  URL:\n    type: url\n"))
	f.Add([]byte(""))
	f.Add([]byte("keys: {}"))

	f.Fuzz(func(t *testing.T, data []byte) {
		var s Schema
		if err := decodeYAML(data, &s); err != nil {
			return
		}

		_ = s.Validate()
		_ = s.SortedKeys()

		for _, key := range s.Keys {
			_ = key.ValidateValue("")
			_ = key.ValidateValue("test")
			_ = key.ValidateValue("123")
			_ = key.ValidateValue("true")
		}
	})
}
