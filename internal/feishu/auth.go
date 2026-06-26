package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// auth manages a cached tenant_access_token.
type auth struct {
	appID     string
	appSecret string
	baseURL   string
	hc        *http.Client

	mu     sync.Mutex
	token  string
	expiry time.Time
}

func newAuth(appID, appSecret, baseURL string, hc *http.Client) *auth {
	return &auth{appID: appID, appSecret: appSecret, baseURL: baseURL, hc: hc}
}

type tenantTokenResp struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"` // seconds
}

// Token returns a valid tenant_access_token, refreshing when near expiry.
func (a *auth) Token(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && time.Now().Before(a.expiry) {
		return a.token, nil
	}
	body, _ := json.Marshal(map[string]string{
		"app_id":     a.appID,
		"app_secret": a.appSecret,
	})
	url := a.baseURL + "/open-apis/auth/v3/tenant_access_token/internal"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := a.hc.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tr tenantTokenResp
	if err := json.Unmarshal(raw, &tr); err != nil {
		return "", fmt.Errorf("auth decode: %w (%s)", err, string(raw))
	}
	if tr.Code != 0 || tr.TenantAccessToken == "" {
		return "", fmt.Errorf("auth failed: code=%d msg=%s", tr.Code, tr.Msg)
	}
	a.token = tr.TenantAccessToken
	// refresh 60s early
	a.expiry = time.Now().Add(time.Duration(tr.Expire-60) * time.Second)
	return a.token, nil
}
