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

# 项目配置
REPO_URL="https://github.com/wosa1402/claude2api.git"
PROJECT_DIR="claude2api"

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

# 检查是否在项目目录中
check_in_project_dir() {
    if [ -f "main.go" ] && [ -f "go.mod" ]; then
        return 0
    else
        return 1
    fi
}

# 克隆项目仓库
clone_repository() {
    print_info "正在克隆项目仓库..."

    if [ -d "$PROJECT_DIR" ]; then
        print_warning "目录 $PROJECT_DIR 已存在"
        read -p "是否删除并重新克隆? [y/N]: " confirm
        if [[ $confirm == [yY] ]]; then
            rm -rf "$PROJECT_DIR"
        else
            print_info "使用现有目录"
            cd "$PROJECT_DIR"
            return 0
        fi
    fi

    git clone "$REPO_URL" "$PROJECT_DIR"
    cd "$PROJECT_DIR"
    print_success "项目克隆完成"
}

# 检查前置条件
check_prerequisites() {
    local check_type="${1:-all}"
    print_info "正在检查前置条件..."

    local missing_deps=()

    # Git 是克隆仓库时必需的
    if [[ "$check_type" == "all" || "$check_type" == "git" ]]; then
        if ! command_exists git; then
            missing_deps+=("git")
        fi
    fi

    # Docker 只在 docker 部署时检查
    if [[ "$check_type" == "all" || "$check_type" == "docker" ]]; then
        if ! command_exists docker; then
            missing_deps+=("docker")
        fi
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "缺少必需的依赖: ${missing_deps[*]}"
        print_info "请先安装缺少的依赖。"
        echo ""
        print_info "安装提示:"
        for dep in "${missing_deps[@]}"; do
            case $dep in
                git)
                    echo "  Git: apt install git -y  或  yum install git -y"
                    ;;
                docker)
                    echo "  Docker: curl -fsSL https://get.docker.com | bash"
                    ;;
            esac
        done
        exit 1
    fi

    print_success "前置条件检查通过"
}

# 创建 .env 文件（如果不存在）
setup_env() {
    print_info "正在设置环境配置..."

    if [ ! -f .env ]; then
        if [ -f .env.example ]; then
            cp .env.example .env
            print_success "已从 .env.example 创建 .env 文件"
            print_warning "请编辑 .env 文件配置您的 Claude Session Key"
            echo ""
            read -p "是否现在编辑 .env 文件? [y/N]: " edit_now
            if [[ $edit_now == [yY] ]]; then
                ${EDITOR:-vi} .env
            else
                print_warning "请稍后手动编辑 .env 文件再启动服务"
            fi
        else
            print_warning "未找到 .env.example，正在创建基础 .env 文件..."
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
            print_success "已创建基础 .env 文件"
            print_warning "请编辑 .env 文件，填入您的 Claude Session Key"
            echo ""
            read -p "是否现在编辑 .env 文件? [y/N]: " edit_now
            if [[ $edit_now == [yY] ]]; then
                ${EDITOR:-vi} .env
            else
                print_warning "请稍后手动编辑 .env 文件: vi .env 或 nano .env"
            fi
        fi
    else
        print_success ".env 文件已存在"
    fi
}

# 使用 Docker 部署
deploy_docker() {
    print_info "正在使用 Docker 部署..."
    echo ""

    # 如果存在旧容器则停止并删除
    if docker ps -a --format '{{.Names}}' | grep -q "^claude2api$"; then
        print_info "发现现有容器，正在停止并删除..."
        docker stop claude2api >/dev/null 2>&1 || true
        docker rm claude2api >/dev/null 2>&1 || true
    fi

    # 构建 Docker 镜像
    print_info "正在构建 Docker 镜像（可能需要几分钟）..."
    docker build -t claude2api:latest .

    # 加载环境变量
    if [ -f .env ]; then
        print_info "正在从 .env 加载环境变量..."
        source .env
    else
        print_error "未找到 .env 文件，请先创建配置文件"
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

    echo ""
    print_success "🎉 Docker 部署完成！"
    echo ""
    print_info "服务地址: http://0.0.0.0:8080"
    print_info "查看日志: docker logs -f claude2api"
    print_info "停止服务: docker stop claude2api"
    print_info "启动服务: docker start claude2api"
}

# 使用 Docker Compose 部署
deploy_docker_compose() {
    print_info "正在使用 Docker Compose 部署..."
    echo ""

    if ! command_exists docker-compose && ! docker compose version >/dev/null 2>&1; then
        print_error "未安装 Docker Compose"
        print_info "安装方法: apt install docker-compose -y"
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
    print_info "正在构建并启动服务（可能需要几分钟）..."
    if docker compose version >/dev/null 2>&1; then
        docker compose up -d --build
    else
        docker-compose up -d --build
    fi

    echo ""
    print_success "🎉 Docker Compose 部署完成！"
    echo ""
    print_info "服务地址: http://0.0.0.0:8080"
    print_info "查看日志: docker compose logs -f"
    print_info "停止服务: docker compose down"
}

# 直接部署（从源码构建）
deploy_direct() {
    print_info "正在从源码部署..."
    echo ""

    if ! command_exists go; then
        print_error "未安装 Go"
        print_info "请先安装 Go 1.23+: https://golang.org/dl/"
        print_info "快速安装: wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz"
        exit 1
    fi

    # 检查 Go 版本
    GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
    REQUIRED_VERSION="1.23"

    if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
        print_error "需要 Go 版本 $REQUIRED_VERSION 或更高，当前版本: $GO_VERSION"
        exit 1
    fi

    # 构建
    print_info "正在构建应用..."
    go build -o claude2api .

    echo ""
    print_success "🎉 构建完成！"
    echo ""
    print_info "启动服务: ./claude2api"
    print_info "后台运行: nohup ./claude2api > claude2api.log 2>&1 &"
    print_warning "提示: 启动前请确保已配置 .env 文件"
}

# 停止服务
stop_service() {
    print_info "正在停止 Claude2API 服务..."
    echo ""

    local stopped=false

    if docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
        docker stop claude2api
        print_success "Docker 容器已停止"
        stopped=true
    fi

    if docker compose ps 2>/dev/null | grep -q claude2api; then
        if docker compose version >/dev/null 2>&1; then
            docker compose down
        else
            docker-compose down
        fi
        print_success "Docker Compose 服务已停止"
        stopped=true
    fi

    # 杀死任何运行中的进程
    if pgrep -f "./claude2api" >/dev/null; then
        pkill -f "./claude2api"
        print_success "直接部署进程已停止"
        stopped=true
    fi

    if [ "$stopped" = true ]; then
        echo ""
        print_success "所有服务已停止"
    else
        print_info "没有发现运行中的服务"
    fi
}

# 显示状态
show_status() {
    print_info "正在检查服务状态..."
    echo ""

    local running=false

    # Docker 容器
    if docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
        print_success "✓ Docker 容器正在运行"
        docker ps --filter "name=claude2api" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
        running=true
    else
        print_info "✗ Docker 容器未运行"
    fi

    echo ""

    # 直接部署
    if pgrep -f "./claude2api" >/dev/null; then
        print_success "✓ 直接部署进程正在运行"
        ps aux | grep "./claude2api" | grep -v grep
        running=true
    else
        print_info "✗ 直接部署进程未运行"
    fi

    echo ""
    if [ "$running" = true ]; then
        print_info "访问地址: http://0.0.0.0:8080"
    fi
}

# 显示使用说明
show_usage() {
    cat << EOF
用法: $0 [命令]

命令:
    docker          使用 Docker 部署（推荐，无需安装 Go）
    compose         使用 Docker Compose 部署
    direct          从源码部署（需要 Go 1.23+）
    stop            停止所有运行中的服务
    status          显示服务状态
    setup           仅设置环境配置
    help            显示此帮助信息

示例:
    $0 docker       # 使用 Docker 部署
    $0 compose      # 使用 Docker Compose 部署
    $0 direct       # 从源码构建并部署
    $0 stop         # 停止所有服务
    $0 status       # 检查服务状态

远程一键部署:
    bash <(curl -fsSL https://raw.githubusercontent.com/wosa1402/claude2api/main/deploy.sh) docker

EOF
}

# 主函数
main() {
    show_banner

    # 如果不在项目目录中且不是 help/status/stop 命令，则需要克隆仓库
    if ! check_in_project_dir; then
        case "${1:-}" in
            help|--help|-h|status|stop|"")
                # 这些命令不需要项目文件
                ;;
            docker|compose)
                # Docker 相关部署需要克隆仓库
                check_prerequisites git
                clone_repository
                ;;
            direct)
                # 直接部署需要克隆仓库
                check_prerequisites git
                clone_repository
                ;;
        esac
    fi

    case "${1:-}" in
        docker)
            check_prerequisites docker
            setup_env
            deploy_docker
            ;;
        compose)
            check_prerequisites docker
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
            # 交互式菜单 - 自动检测环境并智能推荐
            echo ""
            print_info "正在检测您的系统环境..."
            echo ""

            # 检测可用的部署方式
            local has_docker=false
            local has_docker_compose=false
            local has_go=false
            local go_version_ok=false

            if command_exists docker; then
                has_docker=true
                echo -e "${GREEN}✓${NC} Docker 已安装"
            else
                echo -e "${RED}✗${NC} Docker 未安装"
            fi

            if command_exists docker-compose || docker compose version >/dev/null 2>&1; then
                has_docker_compose=true
                echo -e "${GREEN}✓${NC} Docker Compose 已安装"
            else
                echo -e "${RED}✗${NC} Docker Compose 未安装"
            fi

            if command_exists go; then
                has_go=true
                GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
                REQUIRED_VERSION="1.23"
                if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" = "$REQUIRED_VERSION" ]; then
                    go_version_ok=true
                    echo -e "${GREEN}✓${NC} Go $GO_VERSION 已安装"
                else
                    echo -e "${YELLOW}⚠${NC} Go $GO_VERSION 已安装（需要 1.23+）"
                fi
            else
                echo -e "${RED}✗${NC} Go 未安装"
            fi

            echo ""
            print_info "请选择一种部署方式:"
            echo ""

            # 根据环境智能显示选项
            if $has_docker; then
                echo -e "1) Docker ${GREEN}(推荐 - 已检测到)${NC}"
            else
                echo "1) Docker (需要安装)"
            fi

            if $has_docker_compose; then
                echo -e "2) Docker Compose ${GREEN}(已检测到)${NC}"
            else
                echo "2) Docker Compose (需要安装)"
            fi

            if $go_version_ok; then
                echo -e "3) 直接部署 ${GREEN}(源码 - Go 已就绪)${NC}"
            elif $has_go; then
                echo "3) 直接部署 (源码 - 需要升级 Go)"
            else
                echo "3) 直接部署 (源码 - 需要安装 Go 1.23+)"
            fi

            echo "4) 停止服务"
            echo "5) 显示状态"
            echo "6) 退出"
            echo ""

            # 给出智能建议
            if $has_docker; then
                print_success "建议: 选择 Docker 部署（选项 1）- 最简单快捷"
            elif $go_version_ok; then
                print_success "建议: 选择源码部署（选项 3）- Go 环境已就绪"
            else
                print_warning "提示: 建议先安装 Docker 或 Go 环境"
            fi
            echo ""

            read -p "请输入您的选择 [1-6]: " choice

            case $choice in
                1)
                    # 如果不在项目目录，先克隆
                    if ! check_in_project_dir; then
                        check_prerequisites git
                        clone_repository
                    fi
                    check_prerequisites docker
                    setup_env
                    deploy_docker
                    ;;
                2)
                    # 如果不在项目目录，先克隆
                    if ! check_in_project_dir; then
                        check_prerequisites git
                        clone_repository
                    fi
                    check_prerequisites docker
                    setup_env
                    deploy_docker_compose
                    ;;
                3)
                    # 如果不在项目目录，先克隆
                    if ! check_in_project_dir; then
                        check_prerequisites git
                        clone_repository
                    fi
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
                    print_info "退出脚本"
                    exit 0
                    ;;
                *)
                    print_error "无效的选择"
                    echo ""
                    show_usage
                    exit 1
                    ;;
            esac
            ;;
    esac
}

main "$@"
