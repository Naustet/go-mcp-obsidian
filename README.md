# MCP Obsidian Go

A high-performance, zero-dependency Model Context Protocol (MCP) server for Obsidian. This allows AI tools like **Gemini CLI** or **Claude Desktop** to read, search, and write to your Obsidian vault.

## Acknowledgments

This project is a Go port of the original Python implementation by **Markus Pfundstein**: [mcp-obsidian](https://github.com/MarkusPfundstein/mcp-obsidian). We are grateful for his work on the original tool set and logic!

---

## 🚀 Quick Start for Gemini CLI

If you are using `gemini-cli`, follow these steps to get started.

### 1. Installation
You have two choices:

#### Option A: Use a Pre-compiled Binary (Easiest)
1. Download the binary for your system (macOS/Windows/Linux) from the "Releases" section on [GitHub](https://github.com/Naustet/go-mcp-obsidian).
2. **Mac/Linux users:** You must grant execution permissions. Open your terminal and run:
   ```bash
   chmod +x mcp-obsidian-darwin-arm64
   ```
3. You **do not** need to install Go to run the binary.

#### Option B: Build from Source
If you have Go installed:
```bash
go build -o mcp-obsidian main.go
```

### 2. Configure Obsidian
1. Install the **Local REST API** plugin in Obsidian.
2. Enable it and copy your **API Key**.
3. (Optional but recommended) Ensure HTTPS is enabled.

### 3. Add to Gemini CLI
Run the following command to register the server:
```bash
gemini mcp add obsidian "/path/to/your/mcp-obsidian"
```

### 4. Set Environment Variables
Gemini CLI needs your API key. Open your global config file:
`~/.gemini/settings.json` (macOS/Linux) or `%USERPROFILE%\.gemini\settings.json` (Windows).

Edit the `obsidian` entry to include the `env` block:
```json
"mcpServers": {
  "obsidian": {
    "command": "/Users/path/to/mcp-obsidian",
    "env": {
      "OBSIDIAN_API_KEY": "YOUR_API_KEY_HERE",
      "MCP_TRANSPORT": "stdio",
      "OBSIDIAN_SKIP_VERIFY": "true"
    }
  }
}
```
*Note: `OBSIDIAN_SKIP_VERIFY: "true"` allows the server to trust Obsidian's self-signed certificate automatically.*

---

## 💻 Windows Support

This tool works perfectly on Windows. 

### Paths in `settings.json`
On Windows, use forward slashes or escaped backslashes for the command path:
`"command": "C:/Tools/mcp-obsidian.exe"`

### For Developers: Cross-Compiling
If you are on macOS and want to build a version for your Windows colleagues:
```bash
GOOS=windows GOARCH=amd64 go build -o mcp-obsidian-windows-amd64.exe main.go
```

---

## 🛠 Available Tools

Once connected (check status with `/mcp list`), the AI can use:

- `obsidian_list_files_in_vault`: Explore your vault structure.
- `obsidian_get_file_contents`: Read any note.
- `obsidian_simple_search`: Full-text search across all notes.
- `obsidian_put_content`: Create or update notes.
- `obsidian_patch_content`: Precise editing (append/prepend to specific headings).
- `obsidian_get_recent_changes`: See what you've been working on lately.
... and 7 other specialized tools.

## 🔍 Troubleshooting

- **Check Logs:** The server writes detailed handshake logs to `/tmp/mcp-obsidian.log` (macOS/Linux) or `debug.log` in the binary directory (Windows).
- **Red Status?** If `/mcp list` shows red, ensure the `command` path is absolute and the `OBSIDIAN_API_KEY` is correct in `settings.json`.
- **Verify API:** Try running the binary manually in your terminal to see if it prints any error messages before entering the MCP loop.

---

## Why Go?

This port was created to provide a lightweight alternative:
- **No Node.js/Python needed:** No more `node_modules` or complex environment setups.
- **Single Binary:** One file is all you need.
- **Speed:** Native performance with near-instant startup.
