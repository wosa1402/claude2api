#!/bin/bash

# Claude2API One-Click Deployment Script
# Author: yushangxiao
# Description: Automated deployment script supporting multiple deployment methods

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Print colored messages
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Display banner
show_banner() {
    echo -e "${BLUE}"
    cat << "EOF"
   ____ _                 _      ____    _    ____ ___
  / ___| | __ _ _   _  __| | ___|___ \  / \  |  _ \_ _|
 | |   | |/ _` | | | |/ _` |/ _ \ __) |/ _ \ | |_) | |
 | |___| | (_| | |_| | (_| |  __// __// ___ \|  __/| |
  \____|_|\__,_|\__,_|\__,_|\___|_____/_/   \_\_|  |___|

EOF
    echo -e "${NC}"
    echo -e "${GREEN}One-Click Deployment Script${NC}"
    echo ""
}

# Check prerequisites
check_prerequisites() {
    print_info "Checking prerequisites..."

    local missing_deps=()

    if ! command_exists git; then
        missing_deps+=("git")
    fi

    if ! command_exists docker; then
        missing_deps+=("docker")
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        print_info "Please install missing dependencies first."
        exit 1
    fi

    print_success "All prerequisites are met."
}

# Create .env file if not exists
setup_env() {
    print_info "Setting up environment configuration..."

    if [ ! -f .env ]; then
        if [ -f .env.example ]; then
            cp .env.example .env
            print_warning ".env file created from .env.example"
            print_warning "Please edit .env file to configure your settings before running the service."
        else
            print_error ".env.example not found. Creating a basic .env file..."
            cat > .env << 'EOF'
SESSIONS=sk-ant-sid01-xxxx,sk-ant-sid01-yyyy
ADDRESS=0.0.0.0:8080
APIKEY=123
CHAT_DELETE=true
MAX_CHAT_HISTORY_LENGTH=10000
NO_ROLE_PREFIX=false
PROMPT_DISABLE_ARTIFACTS=false
ENABLE_MIRROR_API=false
MIRROR_API_PREFIX=/mirror
EOF
            print_warning "Basic .env file created. Please configure it before running the service."
        fi
    else
        print_success ".env file already exists."
    fi
}

# Deploy with Docker
deploy_docker() {
    print_info "Deploying with Docker..."

    # Stop and remove existing container if exists
    if docker ps -a --format '{{.Names}}' | grep -q "^claude2api$"; then
        print_info "Stopping existing container..."
        docker stop claude2api >/dev/null 2>&1 || true
        docker rm claude2api >/dev/null 2>&1 || true
    fi

    # Build Docker image
    print_info "Building Docker image..."
    docker build -t claude2api:latest .

    # Load environment variables
    if [ -f .env ]; then
        print_info "Loading environment variables from .env..."
        source .env
    else
        print_error ".env file not found. Please create one first."
        exit 1
    fi

    # Run container
    print_info "Starting Docker container..."
    docker run -d \
        -p 8080:8080 \
        --env-file .env \
        --name claude2api \
        --restart unless-stopped \
        claude2api:latest

    print_success "Docker deployment completed!"
    print_info "Service is running on http://0.0.0.0:8080"
    print_info "View logs: docker logs -f claude2api"
}

# Deploy with Docker Compose
deploy_docker_compose() {
    print_info "Deploying with Docker Compose..."

    if ! command_exists docker-compose && ! docker compose version >/dev/null 2>&1; then
        print_error "Docker Compose is not installed."
        exit 1
    fi

    # Create docker-compose.yml if not exists
    if [ ! -f docker-compose.yml ]; then
        print_info "Creating docker-compose.yml..."
        cat > docker-compose.yml << 'EOF'
version: '3'
services:
  claude2api:
    build: .
    container_name: claude2api
    ports:
      - "8080:8080"
    env_file:
      - .env
    restart: unless-stopped
EOF
    fi

    # Deploy
    if docker compose version >/dev/null 2>&1; then
        docker compose up -d --build
    else
        docker-compose up -d --build
    fi

    print_success "Docker Compose deployment completed!"
    print_info "Service is running on http://0.0.0.0:8080"
    print_info "View logs: docker compose logs -f"
}

# Deploy directly (build from source)
deploy_direct() {
    print_info "Deploying from source..."

    if ! command_exists go; then
        print_error "Go is not installed. Please install Go 1.23+ first."
        exit 1
    fi

    # Check Go version
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.23"

    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        print_error "Go version $REQUIRED_VERSION or higher is required. Current version: $GO_VERSION"
        exit 1
    fi

    # Build
    print_info "Building application..."
    go build -o claude2api .

    print_success "Build completed!"
    print_info "Run the service with: ./claude2api"
    print_warning "Make sure .env file is configured before running."
}

# Stop service
stop_service() {
    print_info "Stopping Claude2API service..."

    if docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
        docker stop claude2api
        print_success "Docker container stopped."
    fi

    if docker compose ps 2>/dev/null | grep -q claude2api; then
        if docker compose version >/dev/null 2>&1; then
            docker compose down
        else
            docker-compose down
        fi
        print_success "Docker Compose services stopped."
    fi

    # Kill any running process
    if pgrep -f "./claude2api" >/dev/null; then
        pkill -f "./claude2api"
        print_success "Direct deployment process stopped."
    fi

    print_success "All services stopped."
}

# Show status
show_status() {
    print_info "Checking service status..."
    echo ""

    # Docker container
    if docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
        print_success "Docker container is running"
        docker ps --filter "name=claude2api" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    else
        print_info "Docker container is not running"
    fi

    echo ""

    # Direct deployment
    if pgrep -f "./claude2api" >/dev/null; then
        print_success "Direct deployment process is running"
        ps aux | grep "./claude2api" | grep -v grep
    else
        print_info "Direct deployment process is not running"
    fi
}

# Show usage
show_usage() {
    cat << EOF
Usage: $0 [COMMAND]

Commands:
    docker          Deploy using Docker
    compose         Deploy using Docker Compose
    direct          Deploy from source (requires Go 1.23+)
    stop            Stop all running services
    status          Show service status
    setup           Setup environment configuration only
    help            Show this help message

Examples:
    $0 docker       # Deploy with Docker
    $0 compose      # Deploy with Docker Compose
    $0 direct       # Build and prepare for direct deployment
    $0 stop         # Stop all services
    $0 status       # Check service status

EOF
}

# Main function
main() {
    show_banner

    case "${1:-}" in
        docker)
            check_prerequisites
            setup_env
            deploy_docker
            ;;
        compose)
            check_prerequisites
            setup_env
            deploy_docker_compose
            ;;
        direct)
            setup_env
            deploy_direct
            ;;
        stop)
            stop_service
            ;;
        status)
            show_status
            ;;
        setup)
            setup_env
            ;;
        help|--help|-h)
            show_usage
            ;;
        *)
            print_info "Please select a deployment method:"
            echo ""
            echo "1) Docker (recommended)"
            echo "2) Docker Compose"
            echo "3) Direct deployment (from source)"
            echo "4) Stop services"
            echo "5) Show status"
            echo "6) Exit"
            echo ""
            read -p "Enter your choice [1-6]: " choice

            case $choice in
                1)
                    check_prerequisites
                    setup_env
                    deploy_docker
                    ;;
                2)
                    check_prerequisites
                    setup_env
                    deploy_docker_compose
                    ;;
                3)
                    setup_env
                    deploy_direct
                    ;;
                4)
                    stop_service
                    ;;
                5)
                    show_status
                    ;;
                6)
                    print_info "Exiting..."
                    exit 0
                    ;;
                *)
                    print_error "Invalid choice."
                    show_usage
                    exit 1
                    ;;
            esac
            ;;
    esac
}

main "$@"
