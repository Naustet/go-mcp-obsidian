# MCP Obsidian Go

A high-performance, zero-dependency Model Context Protocol (MCP) server for Obsidian. This allows AI tools like **Gemini CLI**, **Cursor**, or **Claude Desktop** to read, search, and write to your Obsidian vault.

## Acknowledgments

This project is a Go port of the original Python implementation by **Markus Pfundstein**: [mcp-obsidian](https://github.com/MarkusPfundstein/mcp-obsidian). We are grateful for his work on the original tool set and logic!

---

## 🚀 Quick Start for Gemini CLI

The easiest way to use this tool with `gemini-cli` is by adding the configuration directly to your settings. **No `.env` file is required.**

### 1. Installation
1. Download the binary for your system (macOS/Windows/Linux) from the "Releases" section.
2. **Mac users:** You must grant execution permissions and bypass Gatekeeper. Open your terminal and run:
   ```bash
   chmod +x mcp-obsidian-darwin-arm64
   xattr -d com.apple.quarantine mcp-obsidian-darwin-arm64
   ```

### 2. Configure Obsidian
1. Install the **Local REST API** plugin in Obsidian.
2. Enable it and copy your **API Key**.
3. **Security (Highly Recommended):** 
   - Go to `Settings` -> `Local REST API` -> `How to access`.
   - Click the link to download your internal certificate (usually `obsidian-local-rest-api.crt`).
   - Place this file in the same folder as your `mcp-obsidian` binary.

### 3. Add to Gemini CLI
Register the server by running:
```bash
gemini mcp add obsidian "/full/path/to/mcp-obsidian"
```

### 4. Finalize Settings
Open your global config file: `~/.gemini/settings.json` (macOS/Linux) or `%USERPROFILE%\.gemini\settings.json` (Windows). 

Add the `env` block with your key and chosen security method:

```json
"mcpServers": {
  "obsidian": {
    "command": "/full/path/to/mcp-obsidian",
    "env": {
      "OBSIDIAN_API_KEY": "YOUR_API_KEY_HERE",
      "MCP_TRANSPORT": "stdio",
      "OBSIDIAN_PROTOCOL": "https",
      "OBSIDIAN_CA_CERT_FILE": "/full/path/to/obsidian-local-rest-api.crt"
    }
  }
}
```
*Note: If you don't want to use a certificate file, you can set `"OBSIDIAN_SKIP_VERIFY": "true"` instead of `OBSIDIAN_CA_CERT_FILE`.*

---

## 💻 Add to Cursor

Add the following to your global config (`~/.cursor/mcp.json`) or project config (`.cursor/mcp.json`):

```json
{
  "mcpServers": {
    "obsidian": {
      "type": "stdio",
      "command": "/absolute/path/to/mcp-obsidian",
      "env": {
        "OBSIDIAN_API_KEY": "YOUR_KEY_HERE",
        "MCP_TRANSPORT": "stdio",
        "OBSIDIAN_PROTOCOL": "https",
        "OBSIDIAN_SKIP_VERIFY": "true"
      }
    }
  }
}
```

---

## 🛠 Available Tools

Once connected (check status with `/mcp list`), the AI can use:

- `obsidian_list_files_in_vault`: Explore your vault structure.
- `obsidian_get_file_contents`: Read any note.
- `obsidian_simple_search`: Full-text search across all notes.
- `obsidian_put_content`: Create or update notes.
- `obsidian_get_recent_changes`: See what you've been working on lately.

---

## 🔍 Troubleshooting

- **Check Logs:** Handshake logs are at `/tmp/mcp-obsidian.log` (macOS/Linux) or `debug.log` in the binary directory (Windows).
- **Certificate Errors?** Ensure the path to your `.crt` file is absolute, or use `OBSIDIAN_SKIP_VERIFY: "true"`.

---

## For Developers: Cross-Compiling
To build for other platforms from macOS:
```bash
# Windows
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o mcp-obsidian.exe main.go

# Linux
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o mcp-obsidian-linux main.go
```
