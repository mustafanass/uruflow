# Uruflow - Automated Docker Deployment System

[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)
[![Version](https://img.shields.io/badge/Version-0.3.2-green.svg)](https://github.com/mustafanass/uruflow/releases)
[![Go Version](https://img.shields.io/badge/Go-1.21.5+-00ADD8.svg)](https://golang.org/)
[![Docker](https://img.shields.io/badge/Docker-20.10+-2496ED.svg)](https://www.docker.com/)
[![Docker Compose](https://img.shields.io/badge/Docker%20Compose-2.0+-2496ED.svg)](https://docs.docker.com/compose/)

**Uruflow** is a lightweight, automated deployment system that listens for GitHub webhook events and automatically deploys your Docker applications when you push code to specified branches. Built in Go for performance and reliability.

## Features

- Automatic deployments via Git push webhooks
- Full Docker Compose integration
- Multi-branch support for different environments
- SSH authentication for secure Git access
- Real-time monitoring and logging
- Multi-repository management
- Comprehensive CLI interface

## How It Works

```
Push Code → GitHub Webhook → Uruflow → Pull Code → Build & Deploy → Live Application
```

## Quick Start

### Install Dependencies

```bash
sudo apt update && sudo apt upgrade -y
sudo apt install -y git docker.io docker-compose curl wget build-essential

# Install Go
wget https://go.dev/dl/go1.21.5.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc

sudo usermod -aG docker $USER
newgrp docker
```

### Install Uruflow

```bash
cd /opt
sudo git clone https://github.com/mustafanass/uruflow.git
cd uruflow
sudo go build -o uruflow cmd/main.go
sudo cp uruflow /usr/local/bin/
```

### Setup

```bash
# Create directories and user
sudo mkdir -p /etc/uruflow /var/uruflow/repositories /var/log/uruflow
sudo useradd -r -s /bin/bash uruflow
sudo usermod -aG docker uruflow
sudo chown -R uruflow:uruflow /var/uruflow /var/log/uruflow /etc/uruflow

# Setup SSH
ssh-keygen -t rsa -b 4096 -C "uruflow@yourserver.com"
sudo mkdir -p /home/uruflow/.ssh
sudo cp ~/.ssh/id_rsa* /home/uruflow/.ssh/
sudo chown -R uruflow:uruflow /home/uruflow/.ssh
sudo chmod 700 /home/uruflow/.ssh
sudo chmod 600 /home/uruflow/.ssh/id_rsa
```

### Configuration

Create `/etc/uruflow/config.json`:

```json
{
  "repositories": [
    {
      "name": "my-app",
      "git_url": "git@github.com:username/my-app.git",
      "branches": ["main"],
      "compose_file": "docker-compose.yml",
      "auto_deploy": true,
      "enabled": true,
      "branch_config": {
        "main": {
          "project_name": "my-app-production"
        }
      }
    }
  ],
  "settings": {
    "work_dir": "/var/uruflow/repositories",
    "max_concurrent": 2,
    "cleanup_enabled": true,
    "auto_clone": true
  },
  "webhook": {
    "port": "8080",
    "path": "/webhook",
    "secret": ""
  }
}
```
> **⚠️ Note:** Container names should not specify in docker-compose.yml files as they are generated dynamically based on the repository name and branch. Adding static container names may cause deployment conflicts and naming confusion.
### System Service

Create `/etc/systemd/system/uruflow.service`:

```ini
[Unit]
Description=Uruflow Auto-Deployment Service
After=network.target docker.service

[Service]
Type=simple
User=uruflow
Group=uruflow
WorkingDirectory=/home/uruflow
Environment=URUFLOW_CONFIG_DIR=/etc/uruflow
Environment=URUFLOW_LOG_DIR=/var/log/uruflow
Environment=HOME=/home/uruflow
ExecStart=/usr/local/bin/uruflow server
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable uruflow
sudo systemctl start uruflow
```

## CLI Commands

```bash
# Repository management
uruflow repo list                    # List repositories
uruflow repo info                    # Check info of repo
uruflow repo update [my-app]         # Update specific repository

# Deployments
uruflow deploy my-app main           # Manual deployment
uruflow deploy my-app staging        # Deploy specific branch
uruflow deploy status                # Check deployment status

# Monitoring
uruflow status                       # System overview
uruflow logs -f                      # Live logs (real time)
uruflow logs my-app                  # View logs for specific repository
uruflow ssh test                     # Test SSH connection

# Configuration
uruflow config info                  # Show configuration
uruflow config reload               # Reload configuration without restart

# System diagnostics
uruflow system check                 # Check permissions and setup
```

## GitHub Webhook Setup

1. Go to repository Settings → Webhooks
2. Add webhook with URL: `http://YOUR-SERVER-IP:8080/webhook`
3. Set content type: `application/json`
4. Select "Just the push event"
5. Add secret key from config.json

## Service Management

```bash
sudo systemctl start uruflow         # Start service
sudo systemctl stop uruflow          # Stop service
sudo systemctl restart uruflow       # Restart service
sudo systemctl status uruflow        # Check service status
sudo journalctl -u uruflow -f        # View service logs
```

## Configuration Options

### Repository Settings
- `name`: Unique identifier for repository
- `git_url`: SSH Git URL (git@github.com:user/repo.git)
- `branches`: Array of branches to monitor
- `compose_file`: Docker Compose file name
- `auto_deploy`: Enable/disable automatic deployment
- `enabled`: Enable/disable repository
- `branch_config`: Per-branch deployment settings

### System Settings
- `work_dir`: Repository clone directory (default: /var/uruflow/repositories)
- `max_concurrent`: Max concurrent deployments (1-3, default: 2)
- `cleanup_enabled`: Auto-cleanup old containers (default: true)
- `auto_clone`: Auto-clone repositories on startup (default: true)

### Webhook Settings
- `port`: Webhook server port (default: "8080")
- `path`: Webhook endpoint path (default: "/webhook")
- `secret`: GitHub webhook secret

## Multi-Environment Example

```json
{
  "repositories": [
    {
      "name": "my-app",
      "git_url": "git@github.com:username/my-app.git",
      "branches": ["main", "staging", "develop"],
      "branch_config": {
        "main": {
          "project_name": "myapp-prod",
          "compose_file": "docker-compose.prod.yml"
        },
        "staging": {
          "project_name": "myapp-staging",
          "compose_file": "docker-compose.staging.yml"
        }
      }
    }
  ]
}
```

## Troubleshooting

```bash
# Check service
sudo systemctl status uruflow
sudo journalctl -u uruflow -f
sudo journalctl -u uruflow --lines=50

# Test components
uruflow ssh test
uruflow config validate
uruflow system check
docker ps

# Test webhook manually
curl -X POST http://localhost:8080/webhook \
  -H "Content-Type: application/json" \
  -d '{"ref":"refs/heads/main","repository":{"name":"my-app"}}'

# Check container logs
docker logs container-name
```

## Environment Variables

| Variable | Description | Required |
|----------|-------------|----------|
| `URUFLOW_CONFIG_DIR` | Configuration directory | Yes |
| `URUFLOW_LOG_DIR` | Log directory | Yes |

## License

Licensed under GNU General Public License v3.0

## Author

**Mustafa Naseer (Mustafa Gaeed)**
- GitHub: [@mustafanass](https://github.com/mustafanass)
