package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type APIVersion int

const (
	APIV5 APIVersion = iota
	APIV6
)

type Client struct {
	BaseURL    string
	Token      string
	HTTP       *http.Client
	APIVersion APIVersion
}

func New(baseURL, token string) *Client {
	c := &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		Token:   token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}
	c.detectVersion()
	return c
}

func (c *Client) detectVersion() {
	c.APIVersion = APIV5
	resp, err := c.HTTP.Get(c.BaseURL + "/system")
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		if v := resp.Header.Get("X-Graylog-Version"); strings.HasPrefix(v, "6.") {
			c.APIVersion = APIV6
		}
	}
}

func (c *Client) doRequest(method, path string, body any) ([]byte, error) {
	var buf io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", c.BaseURL, path), buf)
	if err != nil { return nil, err }
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Requested-By", "terraform-provider")
	req.Header.Set("Authorization", "Basic "+c.Token)
	resp, err := c.HTTP.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Graylog API error: %s", string(b))
	}
	return io.ReadAll(resp.Body)
}

type Rule struct { Field, Type, Value string }

type Stream struct {
	ID          string `json:"id,omitempty"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Disabled    bool   `json:"disabled"`
	IndexSetID  string `json:"index_set_id,omitempty"`
	Rules       []Rule `json:"rules,omitempty"`
}

func (c *Client) CreateStream(s *Stream) (*Stream, error) {
	path := "/streams"
	if c.APIVersion == APIV6 { path = "/api/streams" }
	resp, err := c.doRequest("POST", path, s); if err != nil { return nil, err }
	var out Stream; _ = json.Unmarshal(resp, &out); return &out, nil
}

func (c *Client) GetStream(id string) (*Stream, error) {
	path := fmt.Sprintf("/streams/%s", id)
	if c.APIVersion == APIV6 { path = fmt.Sprintf("/api/streams/%s", id) }
	resp, err := c.doRequest("GET", path, nil); if err != nil { return nil, err }
	var out Stream; _ = json.Unmarshal(resp, &out); return &out, nil
}

func (c *Client) DeleteStream(id string) error {
	path := fmt.Sprintf("/streams/%s", id)
	if c.APIVersion == APIV6 { path = fmt.Sprintf("/api/streams/%s", id) }
	_, err := c.doRequest("DELETE", path, nil); return err
}

type Input struct {
	ID           string   `json:"id,omitempty"`
	Title        string   `json:"title"`
	Type         string   `json:"type"`
	BindAddress  string   `json:"bind_address,omitempty"`
	Port         int      `json:"port,omitempty"`
	KafkaBrokers []string `json:"kafka_brokers,omitempty"`
	Topic        string   `json:"topic,omitempty"`
	IndexSetID   string   `json:"index_set_id,omitempty"`
}

func (c *Client) CreateInput(in *Input) (*Input, error) {
	path := "/system/inputs"
	if c.APIVersion == APIV6 { path = "/api/system/inputs" }
	resp, err := c.doRequest("POST", path, in); if err != nil { return nil, err }
	var out Input; _ = json.Unmarshal(resp, &out); return &out, nil
}

type IndexSet struct {
	ID                string `json:"id,omitempty"`
	Title             string `json:"title"`
	Description       string `json:"description,omitempty"`
	RotationStrategy  string `json:"rotation_strategy,omitempty"`
	RetentionStrategy string `json:"retention_strategy,omitempty"`
	Shards            int    `json:"shards,omitempty"`
}

func (c *Client) CreateIndexSet(is *IndexSet) (*IndexSet, error) {
	path := "/system/indices/index_sets"
	if c.APIVersion == APIV6 { path = "/api/system/indices/index_sets" }
	resp, err := c.doRequest("POST", path, is); if err != nil { return nil, err }
	var out IndexSet; _ = json.Unmarshal(resp, &out); return &out, nil
}
