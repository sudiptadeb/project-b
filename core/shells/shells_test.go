package shells

import (
	"os"
	"slices"
	"testing"
)

func TestTheListedShellsAreThoseOfThisKindOfDevice(t *testing.T) {
	var listed []string
	for _, s := range Listed() {
		listed = append(listed, s.ID)
	}
	if len(listed) == 0 || listed[0] != "standard" || !Has("minimal") || Has("Minimal") {
		t.Fatalf("listed %v", listed)
	}
	for _, s := range Listed() {
		if s.Phone {
			t.Fatalf("a phone's shell is listed on a desktop: %+v", s)
		}
	}
}

// Every folder of shell/src/apps/shell is a shell the registry has, and every shell the
// registry has is a folder there.
func TestTheRegistryAndTheFoldersNameTheSameShells(t *testing.T) {
	entries, err := os.ReadDir("../../shell/src/apps/shell")
	if err != nil {
		t.Fatal(err)
	}
	var folders, registered []string
	for _, e := range entries {
		if e.IsDir() {
			folders = append(folders, e.Name())
		}
	}
	for _, s := range All() {
		registered = append(registered, s.ID)
	}
	slices.Sort(registered)
	if !slices.Equal(folders, registered) {
		t.Fatalf("the folders are %v, the registry has %v", folders, registered)
	}
}

// A desktop offers no phone shell, a phone only those, and each starts on the first it
// offers: standard on a desktop, mobile on a phone. A desktop loads either named outright;
// a phone doesn't load standard, which the profile templates name.
func TestEachKindOfDeviceOffersItsOwnShells(t *testing.T) {
	defer func(was bool) { OnPhone = was }(OnPhone)
	for _, phone := range []bool{false, true} {
		OnPhone = phone
		for _, s := range Listed() {
			if s.Phone != phone {
				t.Fatalf("on a phone %v, %s is offered", phone, s.ID)
			}
		}
		want := map[bool]string{false: "standard", true: "mobile"}[phone]
		if Default() != want || !Fits(want) || Fits("standard") == phone || !Fits("mobile") || Fits("nothing") {
			t.Fatalf("on a phone %v, the default is %s", phone, Default())
		}
	}
}
