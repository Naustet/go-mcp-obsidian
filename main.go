// Package main implements a Model Context Protocol (MCP) server for Obsidian.
// This server acts as a bridge between MCP clients (such as Claude or Gemini CLI) 
// and Obsidian via the Obsidian Local REST API plugin.
package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// debugLog writes troubleshooting information to /tmp/mcp-obsidian.log.
func debugLog(msg string) {
	logPath := "/tmp/mcp-obsidian.log"
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil { return }
	defer f.Close()
	fmt.Fprintf(f, "[%s] [PID %d] %s\n", strings.Split(fmt.Sprintf("%v", os.Getpid()), " ")[0], os.Getpid(), msg)
}

// --- MCP / JSON-RPC Types ---

type JSONRPCRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *RPCError        `json:"error,omitempty"`
}

type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type CallToolResult struct {
	Content []TextContent `json:"content"`
}

// --- Obsidian Client ---

type ObsidianClient struct {
	APIKey   string
	Protocol string
	Host     string
	Port     int
	BaseURL  string
	client   *http.Client
}

// NewObsidianClient sets up the client with correct authentication and TLS verification.
func NewObsidianClient() *ObsidianClient {
	apiKey := os.Getenv("OBSIDIAN_API_KEY")
	protocol := os.Getenv("OBSIDIAN_PROTOCOL")
	if protocol == "" { protocol = "https" }
	host := os.Getenv("OBSIDIAN_HOST")
	if host == "" { host = "127.0.0.1" }
	portStr := os.Getenv("OBSIDIAN_PORT")
	port := 27124
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil { port = p }
	}

	caCertPool, _ := x509.SystemCertPool()
	if caCertPool == nil { caCertPool = x509.NewCertPool() }

	// Logic to load certificate
	var certLoaded bool
	
	// 1. Try OBSIDIAN_CA_CERT_PEM (direct string)
	if pem := os.Getenv("OBSIDIAN_CA_CERT_PEM"); pem != "" {
		if ok := caCertPool.AppendCertsFromPEM([]byte(pem)); ok {
			debugLog("Loaded certificate from OBSIDIAN_CA_CERT_PEM")
			certLoaded = true
		}
	}
	
	// 2. Try OBSIDIAN_CA_CERT_FILE (file path)
	if !certLoaded {
		if path := os.Getenv("OBSIDIAN_CA_CERT_FILE"); path != "" {
			data, err := os.ReadFile(path)
			if err == nil {
				if ok := caCertPool.AppendCertsFromPEM(data); ok {
					debugLog("Loaded certificate from " + path)
					certLoaded = true
				}
			}
		}
	}
	
	skipVerify := os.Getenv("OBSIDIAN_SKIP_VERIFY") == "true"

	httpClient := &http.Client{}
	if protocol == "https" {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            caCertPool,
				InsecureSkipVerify: skipVerify,
			},
		}
		if skipVerify {
			debugLog("WARNING: TLS verification is disabled (OBSIDIAN_SKIP_VERIFY=true)")
		} else if !certLoaded {
			debugLog("No custom CA certificate loaded. Relying on system trust store.")
		}
	}

	return &ObsidianClient{
		APIKey:   apiKey,
		Protocol: protocol,
		Host:     host,
		Port:     port,
		BaseURL:  fmt.Sprintf("%s://%s:%d", protocol, host, port),
		client:   httpClient,
	}
}

func (c *ObsidianClient) doRequest(method, path string, body io.Reader, headers map[string]string) ([]byte, error) {
	if c.APIKey == "" { return nil, fmt.Errorf("OBSIDIAN_API_KEY is missing") }
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil { return nil, err }
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	for k, v := range headers { req.Header.Set(k, v) }
	resp, err := c.client.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

// --- Obsidian API Methods ---

func (c *ObsidianClient) ListFilesInVault() (interface{}, error) {
	data, err := c.doRequest("GET", "/vault/", nil, nil)
	if err != nil { return nil, err }
	var res struct { Files []string `json:"files"` }
	json.Unmarshal(data, &res)
	return res.Files, nil
}

func (c *ObsidianClient) ListFilesInDir(dirpath string) (interface{}, error) {
	data, err := c.doRequest("GET", "/vault/"+url.PathEscape(dirpath)+"/", nil, nil)
	if err != nil { return nil, err }
	var res struct { Files []string `json:"files"` }
	json.Unmarshal(data, &res)
	return res.Files, nil
}

func (c *ObsidianClient) GetFileContents(filepath string) (string, error) {
	data, err := c.doRequest("GET", "/vault/"+url.PathEscape(filepath), nil, nil)
	if err != nil { return "", err }
	return string(data), nil
}

func (c *ObsidianClient) Search(query string, contextLength int) (interface{}, error) {
	u := fmt.Sprintf("/search/simple/?query=%s&contextLength=%d", url.QueryEscape(query), contextLength)
	data, err := c.doRequest("POST", u, nil, nil)
	if err != nil { return nil, err }
	var res interface{}
	json.Unmarshal(data, &res)
	return res, nil
}

func (c *ObsidianClient) AppendContent(filepath, content string) error {
	_, err := c.doRequest("POST", "/vault/"+url.PathEscape(filepath), strings.NewReader(content), map[string]string{"Content-Type": "text/markdown"})
	return err
}

func (c *ObsidianClient) PutContent(filepath, content string) error {
	_, err := c.doRequest("PUT", "/vault/"+url.PathEscape(filepath), strings.NewReader(content), map[string]string{"Content-Type": "text/markdown"})
	return err
}

func (c *ObsidianClient) PatchContent(filepath, operation, targetType, target, content string) error {
	headers := map[string]string{"Content-Type": "text/markdown", "Operation": operation, "Target-Type": targetType, "Target": url.QueryEscape(target)}
	_, err := c.doRequest("PATCH", "/vault/"+url.PathEscape(filepath), strings.NewReader(content), headers)
	return err
}

func (c *ObsidianClient) DeleteFile(filepath string) error {
	_, err := c.doRequest("DELETE", "/vault/"+url.PathEscape(filepath), nil, nil)
	return err
}

func (c *ObsidianClient) SearchJSON(query interface{}) (interface{}, error) {
	body, _ := json.Marshal(query)
	data, err := c.doRequest("POST", "/search/", bytes.NewReader(body), map[string]string{"Content-Type": "application/vnd.olrapi.jsonlogic+json"})
	if err != nil { return nil, err }
	var res interface{}
	json.Unmarshal(data, &res)
	return res, nil
}

func (c *ObsidianClient) GetPeriodicNote(period, noteType string) (string, error) {
	headers := map[string]string{}
	if noteType == "metadata" { headers["Accept"] = "application/vnd.olrapi.note+json" }
	data, err := c.doRequest("GET", "/periodic/"+period+"/", nil, headers)
	if err != nil { return "", err }
	return string(data), nil
}

func (c *ObsidianClient) GetRecentPeriodicNotes(period string, limit int, includeContent bool) (interface{}, error) {
	u := fmt.Sprintf("/periodic/%s/recent?limit=%d&includeContent=%t", period, limit, includeContent)
	data, err := c.doRequest("GET", u, nil, nil)
	if err != nil { return nil, err }
	var res interface{}
	json.Unmarshal(data, &res)
	return res, nil
}

func (c *ObsidianClient) GetRecentChanges(limit, days int) (interface{}, error) {
	dql := fmt.Sprintf("TABLE file.mtime\nWHERE file.mtime >= date(today) - dur(%d days)\nSORT file.mtime DESC\nLIMIT %d", days, limit)
	data, err := c.doRequest("POST", "/search/", strings.NewReader(dql), map[string]string{"Content-Type": "application/vnd.olrapi.dataview.dql+txt"})
	if err != nil { return nil, err }
	var res interface{}
	json.Unmarshal(data, &res)
	return res, nil
}

// --- Server Definitions ---

var tools = []Tool{
	{Name: "obsidian_list_files_in_vault", Description: "List all files and folders in the root directory.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}},
	{Name: "obsidian_list_files_in_dir", Description: "List files in a specific directory.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"dirpath": map[string]interface{}{"type": "string"}}, "required": []string{"dirpath"}}},
	{Name: "obsidian_get_file_contents", Description: "Get the content of a single file.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"filepath": map[string]interface{}{"type": "string"}}, "required": []string{"filepath"}}},
	{Name: "obsidian_simple_search", Description: "Search for text across the entire vault.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]interface{}{"type": "string"}}, "required": []string{"query"}}},
	{Name: "obsidian_append_content", Description: "Append text to the end of a file.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"filepath": map[string]interface{}{"type": "string"}, "content": map[string]interface{}{"type": "string"}}, "required": []string{"filepath", "content"}}},
	{Name: "obsidian_patch_content", Description: "Advanced editing based on headings or blocks.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"filepath": map[string]interface{}{"type": "string"}, "operation": map[string]interface{}{"type": "string"}, "target_type": map[string]interface{}{"type": "string"}, "target": map[string]interface{}{"type": "string"}, "content": map[string]interface{}{"type": "string"}}, "required": []string{"filepath", "operation", "target_type", "target", "content"}}},
	{Name: "obsidian_put_content", Description: "Create or overwrite a file.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"filepath": map[string]interface{}{"type": "string"}, "content": map[string]interface{}{"type": "string"}}, "required": []string{"filepath", "content"}}},
	{Name: "obsidian_delete_file", Description: "Delete a file.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"filepath": map[string]interface{}{"type": "string"}, "confirm": map[string]interface{}{"type": "boolean"}}, "required": []string{"filepath", "confirm"}}},
	{Name: "obsidian_complex_search", Description: "Complex search using JsonLogic.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"query": map[string]interface{}{"type": "object"}}, "required": []string{"query"}}},
	{Name: "obsidian_batch_get_file_contents", Description: "Get the contents of multiple files simultaneously.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"filepaths": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}}}, "required": []string{"filepaths"}}},
	{Name: "obsidian_get_periodic_note", Description: "Get a daily/weekly note.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"period": map[string]interface{}{"type": "string"}}, "required": []string{"period"}}},
	{Name: "obsidian_get_recent_periodic_notes", Description: "Get the most recent periodic notes.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"period": map[string]interface{}{"type": "string"}}, "required": []string{"period"}}},
	{Name: "obsidian_get_recent_changes", Description: "List recently modified files.", InputSchema: map[string]interface{}{"type": "object", "properties": map[string]interface{}{"limit": map[string]interface{}{"type": "integer"}}}},
}

// handleRequest processes JSON-RPC requests.
func handleRequest(client *ObsidianClient, req JSONRPCRequest) (interface{}, error) {
	switch req.Method {
	case "initialize":
		debugLog("Processing 'initialize'")
		return map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{"tools": map[string]interface{}{"listChanged": false}},
			"serverInfo": map[string]interface{}{"name": "mcp-obsidian-go", "version": "0.1.0"},
		}, nil
	case "notifications/initialized":
		debugLog("Received initialized notification (ignoring per protocol)")
		return nil, nil 
	case "tools/list":
		debugLog("Processing 'tools/list'")
		return map[string]interface{}{"tools": tools}, nil
	case "tools/call":
		var params struct { Name string `json:"name"`; Arguments map[string]interface{} `json:"arguments"` }
		json.Unmarshal(req.Params, &params)
		debugLog("Calling tool: " + params.Name)
		res, err := callTool(client, params.Name, params.Arguments)
		if err != nil { return nil, err }
		jsonRes, _ := json.MarshalIndent(res, "", "  ")
		return CallToolResult{Content: []TextContent{{Type: "text", Text: string(jsonRes)}}}, nil
	default:
		debugLog("Unknown method: " + req.Method)
		return nil, fmt.Errorf("method not found: %s", req.Method)
	}
}

// callTool routes MCP tool calls to the correct Obsidian method.
func callTool(client *ObsidianClient, name string, args map[string]interface{}) (interface{}, error) {
	switch name {
	case "obsidian_list_files_in_vault": return client.ListFilesInVault()
	case "obsidian_list_files_in_dir": return client.ListFilesInDir(args["dirpath"].(string))
	case "obsidian_get_file_contents": return client.GetFileContents(args["filepath"].(string))
	case "obsidian_simple_search": return client.Search(args["query"].(string), 100)
	case "obsidian_append_content": return "Success", client.AppendContent(args["filepath"].(string), args["content"].(string))
	case "obsidian_put_content": return "Success", client.PutContent(args["filepath"].(string), args["content"].(string))
	case "obsidian_patch_content": return "Success", client.PatchContent(args["filepath"].(string), args["operation"].(string), args["target_type"].(string), args["target"].(string), args["content"].(string))
	case "obsidian_delete_file": return "Success", client.DeleteFile(args["filepath"].(string))
	case "obsidian_complex_search": return client.SearchJSON(args["query"])
	case "obsidian_batch_get_file_contents":
		fps, _ := args["filepaths"].([]interface{})
		var sb strings.Builder
		for _, fp := range fps {
			c, e := client.GetFileContents(fp.(string))
			if e != nil { sb.WriteString(fmt.Sprintf("# %s\nError: %v\n---\n", fp, e)) } else { sb.WriteString(fmt.Sprintf("# %s\n%s\n---\n", fp, c)) }
		}
		return sb.String(), nil
	case "obsidian_get_periodic_note": return client.GetPeriodicNote(args["period"].(string), "content")
	case "obsidian_get_recent_periodic_notes": return client.GetRecentPeriodicNotes(args["period"].(string), 5, false)
	case "obsidian_get_recent_changes": return client.GetRecentChanges(10, 90)
	}
	return nil, fmt.Errorf("unknown tool: %s", name)
}

// runStdio implements Stdio transport for MCP.
func runStdio(client *ObsidianClient) {
	debugLog("Starting stdio loop")
	scanner := bufio.NewScanner(os.Stdin)
	// Increase buffer to 1MB to handle large JSON objects.
	const maxCap = 1024 * 1024
	buf := make([]byte, maxCap); scanner.Buffer(buf, maxCap)

	for scanner.Scan() {
		var req JSONRPCRequest
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			debugLog("Unmarshal error: " + err.Error()); continue
		}

		result, err := handleRequest(client, req)
		if req.ID == nil {
			debugLog("Notification handled without response")
			continue
		}

		resp := JSONRPCResponse{JSONRPC: "2.0", ID: req.ID}
		if err != nil {
			resp.Error = &RPCError{Code: -32000, Message: err.Error()}
		} else {
			resp.Result = result
		}
		data, _ := json.Marshal(resp)
		fmt.Println(string(data))
		debugLog("Response sent to stdout")
	}
	debugLog("Stdio loop terminated")
}

// loadEnv loads .env files manually to avoid external dependencies.
func loadEnv(filename string) {
	debugLog("Attempting to load .env...")
	searchPaths := []string{"."}
	exe, err := os.Executable()
	if err == nil { searchPaths = append(searchPaths, filepath.Dir(exe)) }
	for _, p := range searchPaths {
		f, err := os.Open(filepath.Join(p, filename))
		if err == nil {
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				l := strings.TrimSpace(scanner.Text())
				if l == "" || strings.HasPrefix(l, "#") { continue }
				parts := strings.SplitN(l, "=", 2)
				if len(parts) == 2 { os.Setenv(strings.TrimSpace(parts[0]), strings.Trim(strings.TrimSpace(parts[1]), `"'`)) }
			}
			f.Close(); debugLog("Loaded .env from: " + p); break
		}
	}
}

func main() {
	debugLog("--- Server starting ---")
	loadEnv(".env")
	client := NewObsidianClient()
	
	if os.Getenv("MCP_TRANSPORT") == "stdio" {
		runStdio(client)
		return
	}
	
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost { return }
		var req JSONRPCRequest
		json.NewDecoder(r.Body).Decode(&req)
		res, _ := handleRequest(client, req)
		if req.ID != nil {
			json.NewEncoder(w).Encode(JSONRPCResponse{JSONRPC: "2.0", ID: req.ID, Result: res})
		}
	})
	port := os.Getenv("PORT"); if port == "" { port = "8080" }
	fmt.Fprintf(os.Stderr, "Server listening on port %s...\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
