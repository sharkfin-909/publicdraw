package translog

import (
	"encoding/json"
	"os"
	"testing"
)

func rewrite(t *testing.T, path string, records []Record) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, r := range records {
		b, _ := json.Marshal(r)
		f.Write(append(b, '\n'))
	}
}
