package instance

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestSecondCopyHandsOver(t *testing.T) {
	dir := t.TempDir()
	first, err := Claim(dir, nil)
	if err != nil || first == nil {
		t.Fatalf("first copy: %v, %v", first, err)
	}
	got := make(chan []string, 1)
	first.Serve(func(urls []string) { got <- urls })

	want := []string{"https://example.com/", "https://example.org/"}
	second, err := Claim(dir, want)
	if err != nil || second != nil {
		t.Fatalf("second copy should hand over and quit: %v, %v", second, err)
	}
	select {
	case urls := <-got:
		if !reflect.DeepEqual(urls, want) {
			t.Fatalf("owner got %v, want %v", urls, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the owner never got the URLs")
	}

	first.Close()
	third, err := Claim(dir, nil)
	if err != nil || third == nil {
		t.Fatalf("after the owner closed, a new copy should own the folder: %v, %v", third, err)
	}
	third.Close()
}

func TestLongProfilePath(t *testing.T) {
	dir := t.TempDir() + "/" + strings.Repeat("a-long-folder-name/", 8)
	o, err := Claim(dir, nil)
	if err != nil || o == nil {
		t.Fatalf("a %d-byte profile path: %v, %v", len(dir), o, err)
	}
	o.Close()
}
