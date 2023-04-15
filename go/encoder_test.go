package rod

import (
	"bytes"
	"io"
	"os"
	"testing"

	"github.com/anaminus/deep"
)

func TestEncoder(t *testing.T) {
	b, err := os.ReadFile("testdata/sample.rod")
	if err != nil {
		t.Fatalf("%s", err)
		return
	}

	d := NewDecoder(bytes.NewReader(b))
	var v any
	if err := d.Decode(&v); err != nil {
		t.Fatalf("%s", err)
	}

	f, _ := os.Create("testdata/sample.out.rod")
	defer f.Close()

	var buf bytes.Buffer
	mw := io.MultiWriter(&buf, f)

	e := NewEncoder(mw)
	if err := e.Encode(v); err != nil {
		t.Fatalf("%s", err)
	}

	d = NewDecoder(&buf)
	var u any
	if err := d.Decode(&u); err != nil {
		t.Fatalf("%s", err)
	}

	if diffs := deep.Equal(u, v); len(diffs) > 0 {
		for _, d := range diffs {
			t.Log(d)
		}
		t.Errorf("encoded sample file not equal to control")
	}
}
