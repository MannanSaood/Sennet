package db

import "testing"

func TestExpiredKeyRejectedByBothValidationPaths(t *testing.T) {
	database, err := New(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	key, err := database.CreateAPIKey("expired")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := database.conn.Exec(`UPDATE api_keys SET expires_at = datetime('now', '-1 day') WHERE key = ?`, key); err != nil {
		t.Fatal(err)
	}
	for name, validate := range map[string]func(string) (bool, error){"bearer": database.ValidateAPIKey, "signature": database.APIKeyExists} {
		valid, err := validate(key)
		if err != nil || valid {
			t.Fatalf("%s accepted expired key or failed: valid=%v err=%v", name, valid, err)
		}
	}
}
