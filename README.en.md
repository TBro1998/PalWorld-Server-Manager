# Palworld Server Manager

> English | [中文](./README.md) | [日本語](./README.ja.md)

A comprehensive management tool for Palworld dedicated servers with mod support, multi-server management, and a beautiful web UI.

## Features

- 🚀 **One-click Server Management** - Start, stop, restart servers with ease
- 🎮 **Multi-server Support** - Manage multiple servers with independent configs
- 🔧 **Mod Management** - Install and manage Workshop mods via SteamCMD
- 📊 **Real-time Monitoring** - CPU, memory, and player statistics
- 📝 **Live Logs** - Real-time server logs via SSE
- 🌍 **Multi-language** - English, Chinese, Japanese support
- 🔒 **Secure** - JWT authentication and user management
- 📦 **Single Binary** - Frontend embedded in Go binary (~15-25MB)
- 🖥️ **Cross-platform** - Windows and Linux support

## Installation & Usage

### Download & Install

1. Go to the [Releases](https://github.com/TBro1998/PalWorld-Server-Manager/releases) page and download the latest version
2. Extract the downloaded archive to any directory
3. Double-click to run `palworld-server-manager.exe` (Windows)

### First Time Setup

1. After the program starts, your browser will automatically open to `http://127.0.0.1:8080`
2. Create an administrator account on first visit
3. Log in and start managing your Palworld servers

## Main Features

### Server Management
- One-click Palworld dedicated server installation
- Start, stop, restart servers
- Configure server parameters and ports
- View server running status

### Mod Management
- Auto-download and install mods by Workshop ID
- Enable/disable mods with one click
- Manage installed mod list

### Monitoring & Logs
- Real-time server log viewing
- Monitor server CPU and memory usage
- View online player count

## Configuration

The program supports two configuration methods with priority: **config file > environment variables > defaults**

### Method 1: Configuration File (Recommended)

Create a `config.yaml` file in the program directory:

```yaml
# Web interface settings
host: "127.0.0.1"  # Listen address
port: 8080          # Port

# Path settings
steamcmd_path: "./steamcmd"        # SteamCMD installation path
palworld_base_path: "./palworld"   # Palworld server directory

# Database
database_path: "./palworld.db"

# JWT secret (change in production)
jwt_secret: "your-secure-secret-key"
```

Refer to `config.example.yaml` for a complete configuration example.

### Method 2: Environment Variables

If no `config.yaml` file exists, the program will use environment variables:

- `HOST` - Web interface listen address
- `PORT` - Web interface port
- `STEAMCMD_PATH` - SteamCMD installation path
- `PALWORLD_BASE_PATH` - Palworld server installation directory

## Developer Documentation

If you want to contribute or learn about technical details, please refer to:

- [Technical Proposal](./PalWorld_TECHNICAL_PROPOSAL.md) - Detailed technical design
- [Backend Developer Guide](./server/README.md) - Go backend development guide
- [Frontend Developer Guide](./ui/README.md) - Next.js frontend development guide

## License

MIT

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
