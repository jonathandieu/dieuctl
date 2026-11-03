package op

import "testing"

func TestParseItem(t *testing.T) {
	data := []byte(`{
	  "title": "Anthropic",
	  "fields": [
	    {"id":"a","label":"default/platform-production","type":"CONCEALED","value":"k1"},
	    {"id":"b","label":"valid from","type":"DATE","value":1767225600},
	    {"id":"c","label":"credential","type":"CONCEALED"},
	    {"id":"d","label":"notesPlain","type":"STRING","value":"text"}
	  ]
	}`)
	item, err := ParseItem(data)
	if err != nil {
		t.Fatal(err)
	}
	if item.Title != "Anthropic" || len(item.Fields) != 4 {
		t.Fatalf("item = %+v", item)
	}
	if f := item.Fields[0]; f.Label != "default/platform-production" || f.Type != "CONCEALED" || f.Value != "k1" {
		t.Fatalf("field0 = %+v", f)
	}
	if item.Fields[1].Value != "" {
		t.Fatal("non-string values must be dropped")
	}
	if item.Fields[2].Value != "" {
		t.Fatal("absent value must be empty")
	}
}

func TestParseItemRejectsGarbage(t *testing.T) {
	if _, err := ParseItem([]byte("secret-looking-garbage")); err == nil {
		t.Fatal("expected error")
	} else if err.Error() != "op item get: unexpected output" {
		t.Fatalf("error must not echo input, got %q", err)
	}
}
