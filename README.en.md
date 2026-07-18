# Palworld Server Manager

> English | [中文](./README.md) | [日本語](./README.ja.md)

A comprehensive management tool for Palworld dedicated servers with mod support, multi-server management, and a modern web UI.

> ⚠️ **Project Status: In Development**
>
> This project has not been officially released yet and is still under active development. Features may be unstable, subject to change, or incomplete, and it is not recommended for production use. Feedback and testing are welcome — stay tuned for the official release.

## Features

### Implemented

- 🚀 **One-click Server Management** - Start, stop, and restart servers with ease
- 📥 **One-click Server Installation** - Download and install the Palworld dedicated server via SteamCMD
- 🎮 **Multi-server Support** - Manage multiple servers with independent configs, saves, and ports
- ⚙️ **Visual Configuration Editing** - Graphically edit launch arguments and `PalWorldSettings.ini`
- 📝 **Live Logs** - View server logs in real time via SSE (including historical logs)
- 🎛️ **REST API Commands** - broadcast / save / shutdown / kick / ban as an RCON replacement
- 🌍 **Multi-language** - Chinese, English, and Japanese support
- 📦 **Single Binary** - Frontend embedded in the Go binary (~15-25MB)
- 🖥️ **Cross-platform Architecture** - Cross-platform at the code level; currently only Windows is officially supported due to mod constraints

### Planned

- 🔧 **Mod Management** - Enter a Workshop ID to auto-download, install, and toggle mods via SteamCMD
- 🔒 **Authentication** - JWT authentication and user management to protect remote access
- 📊 **Real-time Monitoring** - CPU, memory, and online player statistics
- ⬆️ **Auto-update** - Update detection and one-click update based on GitHub Releases

## Installation & Usage

### Download & Install

1. Go to the [Releases](https://github.com/TBro1998/PalWorld-Server-Manager/releases) page and download the latest version
2. Extract the downloaded archive to any directory
3. Double-click to run `palworld-server-manager.exe` (Windows)

### First Time Setup

1. After the program starts, open `http://127.0.0.1:8080` in your browser (for Docker/remote deployments, visit `http://<host-IP>:8080`)
2. Create an administrator account on first visit
3. Log in and start managing your Palworld servers

### Docker Deployment (Recommended for Linux)

The manager and the Palworld game server run inside the **same container**, with data persisted to a volume so rebuilding the container won't lose saves.

Image available on Docker Hub: [`tbro98/palsm`](https://hub.docker.com/r/tbro98/palsm)

```bash
# 1. Download docker-compose.yml
curl -O https://raw.githubusercontent.com/TBro1998/PalWorld-Server-Manager/main/docker-compose.yml

# 2. Change JWT_SECRET (required)
#    Edit docker-compose.yml and set JWT_SECRET to a strong random value

# 3. Pull and start (image pulled from Docker Hub automatically — no local build needed)
docker compose up -d

# 4. Visit http://<host-IP>:8080, create an admin account, then install/manage servers
```

Key points:

- **Be sure to change `JWT_SECRET` in `docker-compose.yml`** before using it in production.
- SteamCMD and the Palworld server are **auto-downloaded on first run** by the program into the `/data` volume; no manual pre-installation needed.
- Default port mappings: `8080/tcp` (management UI), `8211/udp` (game), `27015/udp` (query).
  If you change a server's `-port` / `-QueryPort` in the UI, update the compose port mappings accordingly.
- The `./psm-data` directory mounts to `/data` in the container and contains the database, SteamCMD, saves, and logs. Back up this directory to back up all data.
- The image is based on Debian (glibc) with the runtime libraries required by SteamCMD and the Palworld Linux server built in; the container runs as the non-root user `steam`.
- To update to the latest version: `docker compose pull && docker compose up -d`

### Native Linux Deployment

Without Docker, you can also run directly on a Linux host (requires x86_64, glibc):

```bash
# Dependencies (Debian/Ubuntu example): SteamCMD is a 32-bit program
sudo dpkg --add-architecture i386 && sudo apt-get update
sudo apt-get install -y ca-certificates lib32gcc-s1 libstdc++6 libstdc++6:i386

# Run (set HOST=0.0.0.0 for external access)
HOST=0.0.0.0 PORT=8080 JWT_SECRET=your-secret ./palworld-server-manager
```

The program automatically creates the `steamclient.so` symlink required by Palworld under `~/.steam/sdk64`; once installation completes, you can start the server.

## Main Features

### Server Management (Implemented)
- One-click Palworld dedicated server installation
- Start, stop, restart servers
- Visually edit server parameters, launch arguments, and ports
- View server running status
- Independent management of multiple servers

### Logs (Implemented)
- View server logs in real time (SSE push)
- View historical logs

### REST API Commands (Implemented)
- broadcast / save / shutdown / kick / ban (RCON replacement)

### Planned
- **Mod Management**: Enter a Workshop ID to auto-download and install mods, enable/disable with one click, manage the installed list
- **Authentication**: Username/password login with JWT session protection
- **System Monitoring**: Server CPU, memory usage, and online player count
- **Auto-update**: Update detection and one-click update based on GitHub Releases

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

[GNU Affero General Public License v3.0](./LICENSE)

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.
