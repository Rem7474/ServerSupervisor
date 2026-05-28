package dispatcher

import "testing"

func TestValidComposeName(t *testing.T) {
	valid := []string{"myapp", "my-app", "my_app", "app.1", "App2", "a"}
	for _, n := range valid {
		if !validComposeName.MatchString(n) {
			t.Errorf("expected %q to be valid", n)
		}
	}
	invalid := []string{"", "-app", ".app", "_app", "my app", "app;rm", "--force", "app/../etc", "app$(x)"}
	for _, n := range invalid {
		if validComposeName.MatchString(n) {
			t.Errorf("expected %q to be invalid", n)
		}
	}
}
