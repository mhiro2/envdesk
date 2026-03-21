package envfile

import "testing"

func FuzzParse(f *testing.F) {
	f.Add([]byte("KEY=value\n"))
	f.Add([]byte("KEY=\"quoted value\"\n"))
	f.Add([]byte("KEY='single quoted'\n"))
	f.Add([]byte("export KEY=value\n"))
	f.Add([]byte("# comment\nKEY=value\n"))
	f.Add([]byte("KEY=value # inline comment\n"))
	f.Add([]byte("KEY=\"value with \\\"escaped\\\"\"\n"))
	f.Add([]byte(""))
	f.Add([]byte("KEY=\n"))
	f.Add([]byte("KEY=value\nKEY2=value2\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		doc, err := Parse(data)
		if err != nil {
			return
		}

		if len(doc.Keys()) != len(doc.Entries) {
			t.Errorf("Keys() length %d != Entries length %d", len(doc.Keys()), len(doc.Entries))
		}

		_ = doc.Bytes()
		_ = doc.Values()

		for _, entry := range doc.Entries {
			if _, ok := doc.Lookup(entry.Key); !ok {
				t.Errorf("Lookup(%q) = false, want true", entry.Key)
			}
		}
	})
}
