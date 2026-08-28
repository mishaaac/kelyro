package platformos

import (
	"errors"
	"reflect"
	"testing"
)

func TestOpenURLUsesNativeOpenerWithoutShell(t *testing.T) {
	tests := []struct {
		goos string
		want command
	}{
		{goos: "linux", want: command{executable: "/bin/xdg-open", args: []string{"https://go.dev/ref/spec?x=1"}}},
		{goos: "darwin", want: command{executable: "/bin/open", args: []string{"https://go.dev/ref/spec?x=1"}}},
		{goos: "windows", want: command{executable: "/bin/rundll32.exe", args: []string{"url.dll,FileProtocolHandler", "https://go.dev/ref/spec?x=1"}}},
	}
	for _, test := range tests {
		t.Run(test.goos, func(t *testing.T) {
			var got command
			service := &Platform{
				goos:   test.goos,
				lookup: func(name string) (string, error) { return "/bin/" + name, nil },
				run:    func(specification command) error { got = specification; return nil },
			}
			if err := service.OpenURL("https://go.dev/ref/spec?x=1"); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("command = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestOpenURLRejectsUnsafeLocatorsAndReportsFailures(t *testing.T) {
	service := &Platform{goos: "linux", lookup: func(string) (string, error) { return "", errors.New("missing") }}
	for _, locator := range []string{"", "file:///tmp/source", "javascript:alert(1)", "https:///missing-host", "https://user:secret@example.test"} {
		if err := service.OpenURL(locator); err == nil {
			t.Errorf("OpenURL(%q) succeeded", locator)
		}
	}
	if err := service.OpenURL("https://example.test"); err == nil {
		t.Fatal("missing opener did not fail")
	}
}
