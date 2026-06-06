package session

import (
	"testing"
)

// Abort removes durable session state so it can no longer be resumed. This
// exercises the Save → Load → Delete lifecycle that `gpt abort` relies on.
func TestDelete_RemovesDurableState(t *testing.T) {
	t.Setenv("HOME", t.TempDir()) // BaseDir() derives from HOME

	rec := &Record{
		Kind: KindGptDo,
		Name: "abort-me",
		GptDo: &GptDoData{
			Request:   "do something",
			Completed: false,
		},
	}
	if err := Save(rec); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := Load("abort-me"); err != nil {
		t.Fatalf("Load after Save: %v", err)
	}

	if err := Delete("abort-me"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := Load("abort-me"); err == nil {
		t.Fatal("session still loadable after Delete; durable state not cleaned")
	}
}
