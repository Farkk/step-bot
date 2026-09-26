package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type ObjectStore struct {
	Endpoint, Bucket, AccessKey, SecretKey string
	Client                                 *http.Client
}

func ObjectStoreFromEnv(endpoint string) *ObjectStore {
	return &ObjectStore{Endpoint: endpoint, Bucket: os.Getenv("S3_BUCKET"), AccessKey: os.Getenv("S3_ACCESS_KEY"), SecretKey: os.Getenv("S3_SECRET_KEY"), Client: &http.Client{Timeout: 20 * time.Second}}
}
func sign(key []byte, value string) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write([]byte(value))
	return h.Sum(nil)
}
func (s *ObjectStore) request(ctx context.Context, method, key string, data []byte, contentType string) (*http.Response, error) {
	if s == nil || s.Bucket == "" || s.AccessKey == "" || s.SecretKey == "" {
		return nil, fmt.Errorf("S3 не настроено")
	}
	endpoint, err := url.Parse(strings.TrimRight(s.Endpoint, "/"))
	if err != nil {
		return nil, err
	}
	path := "/" + s.Bucket
	if key != "" {
		path += "/" + key
	}
	endpoint.Path = path
	sum := sha256.Sum256(data)
	bodyHash := hex.EncodeToString(sum[:])
	now := time.Now().UTC()
	date := now.Format("20060102")
	timestamp := now.Format("20060102T150405Z")
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-amz-date", timestamp)
	req.Header.Set("x-amz-content-sha256", bodyHash)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	canonical := method + "\n" + req.URL.EscapedPath() + "\n\nhost:" + req.URL.Host + "\nx-amz-content-sha256:" + bodyHash + "\nx-amz-date:" + timestamp + "\n\nhost;x-amz-content-sha256;x-amz-date\n" + bodyHash
	hashed := sha256.Sum256([]byte(canonical))
	scope := date + "/us-east-1/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + timestamp + "\n" + scope + "\n" + hex.EncodeToString(hashed[:])
	k := sign([]byte("AWS4"+s.SecretKey), date)
	k = sign(k, "us-east-1")
	k = sign(k, "s3")
	k = sign(k, "aws4_request")
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.AccessKey+"/"+scope+", SignedHeaders=host;x-amz-content-sha256;x-amz-date, Signature="+hex.EncodeToString(sign(k, stringToSign)))
	return s.Client.Do(req)
}
func (s *ObjectStore) Put(ctx context.Context, key string, data []byte, contentType string) error {
	bucket, err := s.request(ctx, http.MethodPut, "", nil, "")
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(bucket.Body, 1024))
	bucket.Body.Close()
	if bucket.StatusCode != 200 && bucket.StatusCode != 409 {
		return fmt.Errorf("S3 bucket: HTTP %d", bucket.StatusCode)
	}
	resp, err := s.request(ctx, http.MethodPut, key, data, contentType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1024))
	if resp.StatusCode != 200 {
		return fmt.Errorf("S3 upload: HTTP %d", resp.StatusCode)
	}
	return nil
}
func (s *ObjectStore) Get(ctx context.Context, key string) ([]byte, error) {
	resp, err := s.request(ctx, http.MethodGet, key, nil, "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("S3 download: HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 5<<20+1))
}
func (s *ObjectStore) Delete(ctx context.Context, key string) {
	resp, err := s.request(ctx, http.MethodDelete, key, nil, "")
	if err == nil {
		resp.Body.Close()
	}
}
