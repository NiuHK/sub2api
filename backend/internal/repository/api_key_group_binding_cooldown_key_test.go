package repository

import "testing"

func TestAPIKeyGroupBindingCooldownKey(t *testing.T) {
	first := apiKeyGroupBindingCooldownKey(12, 34)
	if first != "apikey:group-binding:cooldown:12:34" {
		t.Fatalf("unexpected cooldown key: %q", first)
	}
	if first == apiKeyGroupBindingCooldownKey(123, 4) || first == apiKeyGroupBindingCooldownKey(12, 35) {
		t.Fatal("cooldown key must distinguish API key and group ID pairs")
	}
}
