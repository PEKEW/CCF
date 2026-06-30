package feishu

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// RealClient talks to the Feishu open API.
//
// Notes / MVP limitations:
//   - Content is written into docx documents as plain text paragraph blocks
//     (markdown is not rendered to rich blocks).
//   - UpdateDoc does a full replace: it clears the document's root children
//     (paginated count + batch_delete) and rewrites content. AppendDoc still
//     appends (used for checkpoints/validation logs).
//   - RenameFolder uses the drive PATCH endpoint; if the tenant/API does not
//     support folder rename it logs and returns nil rather than failing a hook.
type RealClient struct {
	baseURL string
	hc      *http.Client
	auth    *auth
}

// NewRealClient constructs a RealClient. baseURL defaults to the public host.
func NewRealClient(appID, appSecret, baseURL string) *RealClient {
	if baseURL == "" {
		baseURL = "https://open.feishu.cn"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	hc := &http.Client{Timeout: 20 * time.Second}
	return &RealClient{
		baseURL: baseURL,
		hc:      hc,
		auth:    newAuth(appID, appSecret, baseURL, hc),
	}
}

// do performs an authenticated JSON request and decodes data into out.
func (c *RealClient) do(ctx context.Context, method, path string, in any, out any) error {
	tok, err := c.auth.Token(ctx)
	if err != nil {
		return err
	}
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("%s %s: decode: %w (%s)", method, path, err, truncate(string(raw)))
	}
	if env.Code != 0 {
		return fmt.Errorf("%s %s: api code=%d msg=%s", method, path, env.Code, env.Msg)
	}
	if out != nil && len(env.Data) > 0 {
		return json.Unmarshal(env.Data, out)
	}
	return nil
}

func truncate(s string) string {
	if len(s) > 300 {
		return s[:300] + "…"
	}
	return s
}

func (c *RealClient) CreateSessionFolder(ctx context.Context, title, parentToken string) (*FolderRef, error) {
	var out struct {
		Token string `json:"token"`
		URL   string `json:"url"`
	}
	in := map[string]string{"name": title, "folder_token": parentToken}
	if err := c.do(ctx, http.MethodPost, "/open-apis/drive/v1/files/create_folder", in, &out); err != nil {
		return nil, err
	}
	return &FolderRef{Token: out.Token, URL: out.URL}, nil
}

func (c *RealClient) RenameFolder(ctx context.Context, folderToken, title string) error {
	// Best-effort: drive v1 has no stable folder-rename endpoint across all
	// tenants. Attempt PATCH; swallow unsupported responses.
	in := map[string]string{"name": title}
	err := c.do(ctx, http.MethodPatch, "/open-apis/drive/v1/files/"+folderToken, in, nil)
	if err != nil {
		// Non-fatal: do not block the session over a rename.
		return nil
	}
	return nil
}

func (c *RealClient) CreateDoc(ctx context.Context, folderToken, title, content string) (*DocRef, error) {
	var out struct {
		Document struct {
			DocumentID string `json:"document_id"`
		} `json:"document"`
	}
	in := map[string]string{"folder_token": folderToken, "title": title}
	if err := c.do(ctx, http.MethodPost, "/open-apis/docx/v1/documents", in, &out); err != nil {
		return nil, err
	}
	docID := out.Document.DocumentID
	url := c.baseURL + "/docx/" + docID
	if content != "" {
		if err := c.appendBlocks(ctx, docID, content); err != nil {
			return &DocRef{Token: docID, URL: url}, err
		}
	}
	return &DocRef{Token: docID, URL: url}, nil
}

func (c *RealClient) UpdateDoc(ctx context.Context, docToken, content string) error {
	// Full replace: clear existing root children, then write fresh content.
	// Falls back to append if clearing fails, so a doc is never left empty.
	if err := c.clearDoc(ctx, docToken); err != nil {
		return c.appendBlocks(ctx, docToken, content)
	}
	return c.appendBlocks(ctx, docToken, content)
}

// rootChildrenCount returns the number of direct children of the document root
// block, paginating through all pages.
func (c *RealClient) rootChildrenCount(ctx context.Context, docID string) (int, error) {
	total := 0
	pageToken := ""
	for {
		path := fmt.Sprintf("/open-apis/docx/v1/documents/%s/blocks/%s/children?page_size=500", docID, docID)
		if pageToken != "" {
			path += "&page_token=" + pageToken
		}
		var out struct {
			Items     []json.RawMessage `json:"items"`
			PageToken string            `json:"page_token"`
			HasMore   bool              `json:"has_more"`
		}
		if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
			return 0, err
		}
		total += len(out.Items)
		if !out.HasMore || out.PageToken == "" {
			break
		}
		pageToken = out.PageToken
	}
	return total, nil
}

// clearDoc removes every direct child block of the document root.
func (c *RealClient) clearDoc(ctx context.Context, docID string) error {
	n, err := c.rootChildrenCount(ctx, docID)
	if err != nil {
		return err
	}
	if n == 0 {
		return nil
	}
	// batch_delete removes children in [start_index, end_index).
	in := map[string]any{"start_index": 0, "end_index": n}
	path := fmt.Sprintf("/open-apis/docx/v1/documents/%s/blocks/%s/children/batch_delete", docID, docID)
	return c.do(ctx, http.MethodDelete, path, in, nil)
}

func (c *RealClient) AppendDoc(ctx context.Context, docToken, content string) error {
	return c.appendBlocks(ctx, docToken, content)
}

// GetDocText returns the document's plain-text content via the docx raw_content
// endpoint.
func (c *RealClient) GetDocText(ctx context.Context, docToken string) (string, error) {
	var out struct {
		Content string `json:"content"`
	}
	path := fmt.Sprintf("/open-apis/docx/v1/documents/%s/raw_content?lang=0", docToken)
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return "", err
	}
	return out.Content, nil
}

// textBlock is a docx text paragraph block (block_type 2).
func textBlock(line string) map[string]any {
	return map[string]any{
		"block_type": 2,
		"text": map[string]any{
			"elements": []map[string]any{
				{"text_run": map[string]any{"content": line}},
			},
		},
	}
}

// appendBlocks appends content (one paragraph block per line) to the document
// root block.
func (c *RealClient) appendBlocks(ctx context.Context, docID, content string) error {
	lines := strings.Split(content, "\n")
	children := make([]map[string]any, 0, len(lines))
	for _, ln := range lines {
		children = append(children, textBlock(ln))
	}
	// docx allows max 50 children per call; chunk it.
	const chunk = 50
	for i := 0; i < len(children); i += chunk {
		end := i + chunk
		if end > len(children) {
			end = len(children)
		}
		in := map[string]any{"index": -1, "children": children[i:end]}
		path := fmt.Sprintf("/open-apis/docx/v1/documents/%s/blocks/%s/children", docID, docID)
		if err := c.do(ctx, http.MethodPost, path, in, nil); err != nil {
			return err
		}
	}
	return nil
}

func (c *RealClient) UploadArtifact(ctx context.Context, folderToken, path string) (*ArtifactRef, error) {
	return nil, fmt.Errorf("UploadArtifact not implemented in MVP")
}
