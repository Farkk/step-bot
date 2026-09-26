package max

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

var ErrInvalidLaunch = errors.New("invalid MAX launch data")

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func VerifyLaunch(raw, token string, now time.Time) (User, error) {
	var user User
	if raw == "" || token == "" || len(raw) > 16*1024 {
		return user, ErrInvalidLaunch
	}
	params, err := url.ParseQuery(raw)
	if err != nil || len(params["hash"]) != 1 || len(params["auth_date"]) != 1 || len(params["user"]) != 1 {
		return user, ErrInvalidLaunch
	}
	keys := make([]string, 0, len(params))
	for key, values := range params {
		if len(values) != 1 || key == "" {
			return user, ErrInvalidLaunch
		}
		if key != "hash" {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, key+"="+params.Get(key))
	}
	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(token))
	signature := hmac.New(sha256.New, secret.Sum(nil))
	signature.Write([]byte(strings.Join(lines, "\n")))
	provided, err := hex.DecodeString(params.Get("hash"))
	if err != nil || !hmac.Equal(provided, signature.Sum(nil)) {
		return user, ErrInvalidLaunch
	}
	issued, err := strconv.ParseInt(params.Get("auth_date"), 10, 64)
	if err != nil || now.Sub(time.Unix(issued, 0)) > time.Hour || time.Unix(issued, 0).After(now.Add(time.Minute)) {
		return user, ErrInvalidLaunch
	}
	if err := json.Unmarshal([]byte(params.Get("user")), &user); err != nil || user.ID <= 0 {
		return User{}, ErrInvalidLaunch
	}
	return user, nil
}

func VerifyContact(phone, authDate, hash string, userID int64, token string, now time.Time) error {
	phone = strings.TrimPrefix(phone, "+")
	issued, err := strconv.ParseInt(authDate, 10, 64)
	if err != nil || token == "" || userID <= 0 || now.Sub(time.Unix(issued, 0)) > time.Hour || time.Unix(issued, 0).After(now.Add(time.Minute)) {
		return ErrInvalidLaunch
	}
	message := "authDate=" + authDate + "\nphone=" + phone + "\nuserId=" + strconv.FormatInt(userID, 10)
	signature := hmac.New(sha256.New, []byte(token))
	signature.Write([]byte(message))
	provided, err := hex.DecodeString(hash)
	if err != nil || !hmac.Equal(provided, signature.Sum(nil)) {
		return ErrInvalidLaunch
	}
	return nil
}
