package max

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

func signedLaunch(t *testing.T, token string, user string, when time.Time) string {
	t.Helper()
	values := url.Values{"auth_date": {timeToString(when)}, "user": {user}}
	keys := []string{"auth_date", "user"}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+values.Get(key))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	signature.Write([]byte(strings.Join(lines, "\n")))
	values.Set("hash", hex.EncodeToString(signature.Sum(nil)))
	return values.Encode()
}

func timeToString(when time.Time) string { return strconv.FormatInt(when.Unix(), 10) }

func TestVerifyLaunch(t *testing.T) {
	now := time.Unix(1780000000, 0)
	data := signedLaunch(t, "test-token", `{"id":42,"first_name":"Иван","last_name":"Петров"}`, now)
	user, err := VerifyLaunch(data, "test-token", now)
	if err != nil || user.ID != 42 || user.FirstName != "Иван" || user.LastName != "Петров" {
		t.Fatalf("user=%+v err=%v", user, err)
	}
	for name, candidate := range map[string]string{
		"changed name":   strings.Replace(data, "%D0%98%D0%B2%D0%B0%D0%BD", "%D0%9F%D0%B5%D1%82%D1%80", 1),
		"duplicate hash": data + "&hash=bad",
		"expired":        signedLaunch(t, "test-token", `{"id":42}`, now.Add(-2*time.Hour)),
	} {
		if _, err := VerifyLaunch(candidate, "test-token", now); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
}

func TestVerifyContact(t *testing.T) {
	userID := int64(42)
	phone := "79991234567"
	authDate := "1780000000"
	signature := hmac.New(sha256.New, []byte("test-token"))
	signature.Write([]byte("authDate=" + authDate + "\nphone=" + phone + "\nuserId=42"))
	hash := hex.EncodeToString(signature.Sum(nil))
	if err := VerifyContact(phone, authDate, hash, userID, "test-token", time.Unix(1780000000, 0)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyContact("79990000000", authDate, hash, userID, "test-token", time.Unix(1780000000, 0)); err == nil {
		t.Fatal("accepted changed phone")
	}
}
