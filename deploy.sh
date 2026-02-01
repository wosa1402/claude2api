#!/bin/bash

# Claude2API 一键部署脚本
# 作者: yushangxiao
# 描述: 支持多种部署方式的自动化部署脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # 无颜色

# 打印彩色消息
print_info() {
    echo -e "${BLUE}[信息]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[成功]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[警告]${NC} $1"
}

print_error() {
    echo -e "${RED}[错误]${NC} $1"
}

# 检查命令是否存在
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# 显示横幅
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
    echo -e "${GREEN}一键部署脚本${NC}"
    echo ""
}

# 检查前置条件
check_prerequisites() {
    print_info "正在检查前置条件..."

    local missing_deps=()

    if ! command_exists git; then
        missing_deps+=("git")
    fi

    if ! command_exists docker; then
        missing_deps+=("docker")
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "缺少必需的依赖: ${missing_deps[*]}"
        print_info "请先安装缺少的依赖。"
        exit 1
    fi

    print_success "所有前置条件已满足。"
}

# 创建 .env 文件（如果不存在）
setup_env() {
    print_info "正在设置环境配置..."

    if [ ! -f .env ]; then
        if [ -f .env.example ]; then
            cp .env.example .env
            print_warning "已从 .env.example 创建 .env 文件"
            print_warning "请在运行服务之前编辑 .env 文件以配置您的设置。"
        else
            print_error "未找到 .env.example。正在创建基础 .env 文件..."
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
            print_warning "已创建基础 .env 文件。请在运行服务前进行配置。"
        fi
    else
        print_success ".env 文件已存在。"
    fi
}

# 使用 Docker 部署
deploy_docker() {
    print_info "正在使用 Docker 部署..."

    # 如果存在旧容器则停止并删除
    if docker ps -a --format '{{.Names}}' | grep -q "^claude2api$"; then
        print_info "正在停止现有容器..."
        docker stop claude2api >/dev/null 2>&1 || true
        docker rm claude2api >/dev/null 2>&1 || true
    fi

    # 构建 Docker 镜像
    print_info "正在构建 Docker 镜像..."
    docker build -t claude2api:latest .

    # 加载环境变量
    if [ -f .env ]; then
        print_info "正在从 .env 加载环境变量..."
        source .env
    else
        print_error "未找到 .env 文件。请先创建一个。"
        exit 1
    fi

    # 运行容器
    print_info "正在启动 Docker 容器..."
    docker run -d \
        -p 8080:8080 \
        --env-file .env \
        --name claude2api \
        --restart unless-stopped \
        claude2api:latest

    print_success "Docker 部署完成！"
    print_info "服务正在运行: http://0.0.0.0:8080"
    print_info "查看日志: docker logs -f claude2api"
}

# 使用 Docker Compose 部署
deploy_docker_compose() {
    print_info "正在使用 Docker Compose 部署..."

    if ! command_exists docker-compose && ! docker compose version >/dev/null 2>&1; then
        print_error "未安装 Docker Compose。"
        exit 1
    fi

    # 创建 docker-compose.yml（如果不存在）
    if [ ! -f docker-compose.yml ]; then
        print_info "正在创建 docker-compose.yml..."
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

    # 部署
    if docker compose version >/dev/null 2>&1; then
        docker compose up -d --build
    else
        docker-compose up -d --build
    fi

    print_success "Docker Compose 部署完成！"
    print_info "服务正在运行: http://0.0.0.0:8080"
    print_info "查看日志: docker compose logs -f"
}

# 直接部署（从源码构建）
deploy_direct() {
    print_info "正在从源码部署..."

    if ! command_exists go; then
        print_error "未安装 Go。请先安装 Go 1.23+。"
        exit 1
    fi

    # 检查 Go 版本
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.23"

    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        print_error "需要 Go 版本 $REQUIRED_VERSION 或更高。当前版本: $GO_VERSION"
        exit 1
    fi

    # 构建
    print_info "正在构建应用..."
    go build -o claude2api .

    print_success "构建完成！"
    print_info "运行服务: ./claude2api"
    print_warning "运行前请确保已配置 .env 文件。"
}

# 停止服务
stop_service() {
    print_info "正在停止 Claude2API 服务..."

    if docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
        docker stop claude2api
        print_success "Docker 容器已停止。"
    fi

    if docker compose ps 2>/dev/null | grep -q claude2api; then
        if docker compose version >/dev/null 2>&1; then
            docker compose down
        else
            docker-compose down
        fi
        print_success "Docker Compose 服务已停止。"
    fi

    # 杀死任何运行中的进程
    if pgrep -f "./claude2api" >/dev/null; then
        pkill -f "./claude2api"
        print_success "直接部署进程已停止。"
    fi

    print_success "所有服务已停止。"
}

# 显示状态
show_status() {
    print_info "正在检查服务状态..."
    echo ""

    # Docker 容器
    if docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
        print_success "Docker 容器正在运行"
        docker ps --filter "name=claude2api" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
    else
        print_info "Docker 容器未运行"
    fi

    echo ""

    # 直接部署
    if pgrep -f "./claude2api" >/dev/null; then
        print_success "直接部署进程正在运行"
        ps aux | grep "./claude2api" | grep -v grep
    else
        print_info "直接部署进程未运行"
    fi
}

# 显示使用说明
show_usage() {
    cat << EOF
用法: $0 [命令]

命令:
    docker          使用 Docker 部署
    compose         使用 Docker Compose 部署
    direct          从源码部署 (需要 Go 1.23+)
    stop            停止所有运行中的服务
    status          显示服务状态
    setup           仅设置环境配置
    help            显示此帮助信息

示例:
    $0 docker       # 使用 Docker 部署
    $0 compose      # 使用 Docker Compose 部署
    $0 direct       # 构建并准备直接部署
    $0 stop         # 停止所有服务
    $0 status       # 检查服务状态

EOF
}

# 主函数
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
            print_info "请选择一种部署方式:"
            echo ""
            echo "1) Docker (推荐)"
            echo "2) Docker Compose"
            echo "3) 直接部署 (从源码)"
            echo "4) 停止服务"
            echo "5) 显示状态"
            echo "6) 退出"
            echo ""
            read -p "请输入您的选择 [1-6]: " choice

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
                    print_info "正在退出..."
                    exit 0
                    ;;
                *)
                    print_error "无效的选择。"
                    show_usage
                    exit 1
                    ;;
            esac
            ;;
    esac
}

main "$@"
