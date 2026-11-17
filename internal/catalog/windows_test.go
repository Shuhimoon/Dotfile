package catalog

import (
	"reflect"
	"testing"
)

func TestWindowsCatalog(t *testing.T) {
	if got, want := IDsFor("windows"), []ID{"wezterm", "python", "git"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("windows ids %v want %v", got, want)
	}
	if !reflect.DeepEqual(IDsFor("linux"), IDs()) || !reflect.DeepEqual(IDsFor("darwin"), IDs()) {
		t.Fatal("linux/darwin must keep the full catalog")
	}
	if it, ok := ItemByID("wezterm"); !ok || it.Name != "wezterm" {
		t.Fatalf("ItemByID(wezterm) = %v %v", it, ok)
	}
}
