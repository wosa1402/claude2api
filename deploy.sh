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

# 自动安装 Git
auto_install_git() {
    print_info "正在自动安装 Git..."

    if command_exists apt-get; then
        apt-get update && apt-get install -y git
    elif command_exists yum; then
        yum install -y git
    elif command_exists dnf; then
        dnf install -y git
    else
        print_error "无法识别的包管理器，请手动安装 Git"
        return 1
    fi

    if command_exists git; then
        print_success "Git 安装成功"
        return 0
    else
        print_error "Git 安装失败"
        return 1
    fi
}

# 自动安装 Docker
auto_install_docker() {
    print_info "正在自动安装 Docker..."
    print_warning "这可能需要几分钟时间..."

    if curl -fsSL https://get.docker.com | bash; then
        print_success "Docker 安装成功"

        # 启动 Docker 服务
        if command_exists systemctl; then
            systemctl start docker
            systemctl enable docker
            print_info "Docker 服务已启动并设置为开机自启"
        fi

        return 0
    else
        print_error "Docker 安装失败"
        return 1
    fi
}

# 自动安装 Go
auto_install_go() {
    print_info "正在自动安装 Go 1.23..."
    print_warning "这可能需要几分钟时间..."

    local GO_VERSION="1.23.0"
    local ARCH=$(uname -m)

    # 确定架构
    case $ARCH in
        x86_64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        *)
            print_error "不支持的架构: $ARCH"
            return 1
            ;;
    esac

    local GO_TAR="go${GO_VERSION}.linux-${ARCH}.tar.gz"
    local GO_URL="https://go.dev/dl/${GO_TAR}"

    # 下载 Go
    print_info "正在下载 Go ${GO_VERSION}..."
    if ! wget -q --show-progress "$GO_URL" -O "/tmp/${GO_TAR}"; then
        print_error "下载失败"
        return 1
    fi

    # 删除旧版本
    rm -rf /usr/local/go

    # 解压安装
    print_info "正在安装..."
    tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm "/tmp/${GO_TAR}"

    # 设置环境变量
    if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
    fi

    export PATH=$PATH:/usr/local/go/bin

    if command_exists go; then
        print_success "Go $(go version | awk '{print $3}') 安装成功"
        return 0
    else
        print_error "Go 安装失败"
        return 1
    fi
}

# 检查前置条件（带自动安装选项）
check_prerequisites() {
    local check_type="${1:-all}"
    local auto_install="${2:-false}"

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

    # Go 只在源码部署时检查
    if [[ "$check_type" == "go" ]]; then
        if ! command_exists go; then
            missing_deps+=("go")
        else
            GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
            REQUIRED_VERSION="1.23"
            if [ "$(printf '%s\n' "$REQUIRED_VERSION" "$GO_VERSION" | sort -V | head -n1)" != "$REQUIRED_VERSION" ]; then
                missing_deps+=("go")
            fi
        fi
    fi

    if [ ${#missing_deps[@]} -ne 0 ]; then
        print_error "缺少必需的依赖: ${missing_deps[*]}"
        echo ""

        # 如果启用自动安装，询问用户
        if [[ "$auto_install" == "true" ]]; then
            read -p "是否自动安装缺失的依赖? [Y/n]: " confirm
            confirm=${confirm:-Y}

            if [[ $confirm == [yY] ]]; then
                for dep in "${missing_deps[@]}"; do
                    case $dep in
                        git)
                            auto_install_git || exit 1
                            ;;
                        docker)
                            auto_install_docker || exit 1
                            ;;
                        go)
                            auto_install_go || exit 1
                            ;;
                    esac
                done
                print_success "所有依赖安装完成"
                return 0
            fi
        fi

        # 手动安装提示
        print_info "手动安装提示:"
        for dep in "${missing_deps[@]}"; do
            case $dep in
                git)
                    echo "  Git: apt install git -y  或  yum install git -y"
                    ;;
                docker)
                    echo "  Docker: curl -fsSL https://get.docker.com | bash"
                    ;;
                go)
                    echo "  Go: wget https://go.dev/dl/go1.23.0.linux-amd64.tar.gz"
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

    # 询问是否配置为 systemd 服务
    if command_exists systemctl; then
        read -p "是否配置为 systemd 服务（推荐，开机自启）? [Y/n]: " setup_systemd
        setup_systemd=${setup_systemd:-Y}

        if [[ $setup_systemd == [yY] ]]; then
            setup_systemd_service
        else
            print_info "手动启动服务: ./claude2api"
            print_info "后台运行: nohup ./claude2api > claude2api.log 2>&1 &"
            print_warning "提示: 启动前请确保已配置 .env 文件"
        fi
    else
        print_warning "未检测到 systemd，将使用传统方式运行"
        print_info "手动启动服务: ./claude2api"
        print_info "后台运行: nohup ./claude2api > claude2api.log 2>&1 &"
        print_warning "提示: 启动前请确保已配置 .env 文件"
    fi
}

# 配置 systemd 服务
setup_systemd_service() {
    local current_dir=$(pwd)
    local current_user=$(whoami)

    print_info "正在配置 systemd 服务..."

    # 确保工作目录存在
    if [ ! -d "$current_dir" ]; then
        print_error "工作目录不存在: $current_dir"
        return 1
    fi

    # 创建 systemd 服务文件
    cat > /tmp/claude2api.service << EOF
[Unit]
Description=Claude2API Service
After=network.target

[Service]
Type=simple
User=$current_user
WorkingDirectory=$current_dir
ExecStart=$current_dir/claude2api
Restart=always
RestartSec=5
StandardOutput=append:$current_dir/claude2api.log
StandardError=append:$current_dir/claude2api.log

# 环境变量保护
Environment="PWD=$current_dir"

[Install]
WantedBy=multi-user.target
EOF

    # 复制到 systemd 目录
    if [ "$current_user" = "root" ]; then
        mv /tmp/claude2api.service /etc/systemd/system/claude2api.service
    else
        print_info "需要 root 权限安装 systemd 服务"
        sudo mv /tmp/claude2api.service /etc/systemd/system/claude2api.service
    fi

    # 重载 systemd 配置
    if [ "$current_user" = "root" ]; then
        systemctl daemon-reload
        systemctl enable claude2api
        systemctl start claude2api
    else
        sudo systemctl daemon-reload
        sudo systemctl enable claude2api
        sudo systemctl start claude2api
    fi

    echo ""
    print_success "🎉 systemd 服务配置完成！"
    echo ""
    print_info "服务地址: http://0.0.0.0:8080"
    print_info "查看状态: systemctl status claude2api"
    print_info "查看日志: journalctl -u claude2api -f"
    print_info "停止服务: systemctl stop claude2api"
    print_info "启动服务: systemctl start claude2api"
    print_info "重启服务: systemctl restart claude2api"
}

# 更新项目
update_project() {
    print_info "正在更新 Claude2API..."
    echo ""

    # 检查是否在项目目录
    if ! check_in_project_dir; then
        print_error "请在项目目录中运行更新命令"
        exit 1
    fi

    # 检查是否是 git 仓库
    if [ ! -d ".git" ]; then
        print_error "当前目录不是 git 仓库"
        exit 1
    fi

    # 保存当前的 .env 文件
    if [ -f .env ]; then
        print_info "备份配置文件..."
        cp .env .env.backup
    fi

    # 拉取最新代码
    print_info "拉取最新代码..."
    git fetch origin
    git pull origin main

    # 恢复 .env 文件
    if [ -f .env.backup ]; then
        mv .env.backup .env
        print_success "配置文件已恢复"
    fi

    echo ""
    print_success "🎉 更新完成！"
    echo ""

    # 询问是否重新部署
    read -p "是否重新部署服务? [Y/n]: " redeploy
    redeploy=${redeploy:-Y}

    if [[ $redeploy == [yY] ]]; then
        echo ""
        print_info "检测当前部署方式..."

        # 检测部署方式并重新部署
        if command_exists systemctl && systemctl is-active --quiet claude2api; then
            print_info "检测到 systemd 服务，正在重新部署..."
            check_prerequisites go true
            go build -o claude2api .
            if [ "$(whoami)" = "root" ]; then
                systemctl restart claude2api
            else
                sudo systemctl restart claude2api
            fi
            print_success "systemd 服务已重启"
        elif docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
            print_info "检测到 Docker 容器，正在重新部署..."
            check_prerequisites docker true
            docker stop claude2api
            docker rm claude2api
            docker build -t claude2api:latest .
            docker run -d \
                -p 8080:8080 \
                --env-file .env \
                --name claude2api \
                --restart unless-stopped \
                claude2api:latest
            print_success "Docker 容器已重新部署"
        elif docker compose ps 2>/dev/null | grep -q claude2api; then
            print_info "检测到 Docker Compose，正在重新部署..."
            check_prerequisites docker true
            if docker compose version >/dev/null 2>&1; then
                docker compose down
                docker compose up -d --build
            else
                docker-compose down
                docker-compose up -d --build
            fi
            print_success "Docker Compose 服务已重新部署"
        else
            print_warning "未检测到运行中的服务"
            print_info "请手动重新部署服务"
        fi
    fi
}

# 安装全局命令
install_global_command() {
    local script_path=$(pwd)/deploy.sh
    local bin_path="/usr/local/bin/claude2api"

    print_info "正在安装全局命令..."

    # 检查脚本是否存在
    if [ ! -f "$script_path" ]; then
        print_error "deploy.sh 脚本不存在"
        return 1
    fi

    # 创建软链接
    if [ "$(whoami)" = "root" ]; then
        ln -sf "$script_path" "$bin_path"
    else
        print_info "需要 root 权限创建全局命令"
        sudo ln -sf "$script_path" "$bin_path"
    fi

    # 确保脚本可执行
    chmod +x "$script_path"

    if command_exists claude2api; then
        print_success "全局命令安装成功！"
        echo ""
        print_info "现在可以在任何位置使用以下命令:"
        echo "  claude2api status   # 查看服务状态"
        echo "  claude2api stop     # 停止服务"
        echo "  claude2api update   # 更新项目"
        echo "  claude2api help     # 查看帮助"
        return 0
    else
        print_error "全局命令安装失败"
        return 1
    fi
}

# 停止服务
stop_service() {
    print_info "正在停止 Claude2API 服务..."
    echo ""

    local stopped=false

    # 停止 systemd 服务
    if command_exists systemctl && systemctl is-active --quiet claude2api; then
        if [ "$(whoami)" = "root" ]; then
            systemctl stop claude2api
        else
            sudo systemctl stop claude2api
        fi
        print_success "systemd 服务已停止"
        stopped=true
    fi

    # 停止 Docker 容器
    if docker ps --format '{{.Names}}' | grep -q "^claude2api$"; then
        docker stop claude2api
        print_success "Docker 容器已停止"
        stopped=true
    fi

    # 停止 Docker Compose
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

# 卸载服务
uninstall_service() {
    print_warning "========== 完全卸载 Claude2API =========="
    echo ""

    print_info "此操作将完全删除:"
    echo "  • 所有运行中的服务"
    echo "  • systemd 服务配置"
    echo "  • Docker 容器和镜像"
    echo "  • 全局命令"
    echo "  • 项目文件和配置"
    echo ""

    print_warning "警告: 此操作不可逆！"
    echo ""
    read -p "确定要完全卸载吗? 请输入 'yes' 确认: " confirm

    if [ "$confirm" != "yes" ]; then
        print_info "取消卸载"
        return 0
    fi

    echo ""
    local current_dir=$(pwd)

    # 1. 停止所有服务
    print_info "[1/5] 停止所有服务..."

    # 停止 systemd 服务
    if command_exists systemctl && systemctl is-active --quiet claude2api 2>/dev/null; then
        if [ "$(whoami)" = "root" ]; then
            systemctl stop claude2api 2>/dev/null || true
        else
            sudo systemctl stop claude2api 2>/dev/null || true
        fi
        print_success "systemd 服务已停止"
    fi

    # 停止 Docker 容器
    if command_exists docker && docker ps --format '{{.Names}}' 2>/dev/null | grep -q "^claude2api$"; then
        docker stop claude2api 2>/dev/null || true
        print_success "Docker 容器已停止"
    fi

    # 停止 Docker Compose
    if command_exists docker && docker compose ps 2>/dev/null | grep -q claude2api; then
        docker compose down 2>/dev/null || docker-compose down 2>/dev/null || true
        print_success "Docker Compose 已停止"
    fi

    # 杀死直接运行的进程
    if pgrep -f "claude2api" >/dev/null 2>&1; then
        pkill -f "claude2api" 2>/dev/null || true
        print_success "进程已终止"
    fi

    # 2. 删除 systemd 服务
    print_info "[2/5] 删除 systemd 服务..."
    if command_exists systemctl && systemctl list-unit-files 2>/dev/null | grep -q claude2api.service; then
        if [ "$(whoami)" = "root" ]; then
            systemctl disable claude2api 2>/dev/null || true
            rm -f /etc/systemd/system/claude2api.service
            systemctl daemon-reload
        else
            sudo systemctl disable claude2api 2>/dev/null || true
            sudo rm -f /etc/systemd/system/claude2api.service
            sudo systemctl daemon-reload
        fi
        print_success "systemd 服务已删除"
    else
        print_info "无 systemd 服务"
    fi

    # 3. 删除 Docker 容器和镜像
    print_info "[3/5] 删除 Docker 容器和镜像..."
    if command_exists docker; then
        # 删除容器
        if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -q "^claude2api$"; then
            docker rm -f claude2api 2>/dev/null || true
            print_success "Docker 容器已删除"
        fi

        # 删除镜像
        if docker images 2>/dev/null | grep -q "claude2api"; then
            docker rmi -f claude2api:latest 2>/dev/null || true
            print_success "Docker 镜像已删除"
        fi
    else
        print_info "未安装 Docker"
    fi

    # 4. 删除全局命令
    print_info "[4/5] 删除全局命令..."
    if [ -L /usr/local/bin/claude2api ] || [ -f /usr/local/bin/claude2api ]; then
        if [ "$(whoami)" = "root" ]; then
            rm -f /usr/local/bin/claude2api
        else
            sudo rm -f /usr/local/bin/claude2api
        fi
        print_success "全局命令已删除"
    else
        print_info "无全局命令"
    fi

    # 5. 删除项目文件
    print_info "[5/5] 删除项目文件..."
    if check_in_project_dir; then
        cd /tmp
        rm -rf "$current_dir"
        print_success "项目目录已删除: $current_dir"
    else
        print_info "不在项目目录中，跳过文件删除"
    fi

    echo ""
    print_success "========================================="
    print_success "🎉 Claude2API 已完全卸载！"
    print_success "========================================="
}

# 显示状态
show_status() {
    print_info "正在检查服务状态..."
    echo ""

    local running=false

    # systemd 服务
    if command_exists systemctl && systemctl list-unit-files | grep -q claude2api.service; then
        if systemctl is-active --quiet claude2api; then
            print_success "✓ systemd 服务正在运行"
            systemctl status claude2api --no-pager | head -n 10
            running=true
        else
            print_info "✗ systemd 服务未运行（已安装但未启动）"
        fi
        echo ""
    fi

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
    update          更新项目到最新版本
    install         安装全局命令（可在任何位置使用 claude2api）
    stop            停止所有运行中的服务
    status          显示服务状态
    uninstall       卸载服务（删除配置、容器、镜像等）
    setup           仅设置环境配置
    help            显示此帮助信息

示例:
    $0 docker       # 使用 Docker 部署
    $0 compose      # 使用 Docker Compose 部署
    $0 direct       # 从源码构建并部署
    $0 update       # 更新项目并重新部署
    $0 install      # 安装全局命令
    $0 stop         # 停止所有服务
    $0 status       # 检查服务状态
    $0 uninstall    # 完全卸载服务

远程一键部署:
    bash <(curl -fsSL https://raw.githubusercontent.com/wosa1402/claude2api/main/deploy.sh) docker

安装全局命令后:
    claude2api status      # 查看服务状态
    claude2api update      # 更新项目
    claude2api stop        # 停止服务
    claude2api uninstall   # 卸载服务

EOF
}

# 主函数
main() {
    show_banner

    # 如果不在项目目录中且不是 help/status/stop 命令，则需要克隆仓库
    if ! check_in_project_dir; then
        case "${1:-}" in
            help|--help|-h|status|stop|uninstall|"")
                # 这些命令不需要项目文件
                ;;
            docker|compose)
                # Docker 相关部署需要克隆仓库
                check_prerequisites git true
                clone_repository
                ;;
            direct)
                # 直接部署需要克隆仓库
                check_prerequisites git true
                clone_repository
                ;;
        esac
    fi

    case "${1:-}" in
        docker)
            check_prerequisites docker true
            setup_env
            deploy_docker
            ;;
        compose)
            check_prerequisites docker true
            setup_env
            deploy_docker_compose
            ;;
        direct)
            check_prerequisites go true
            setup_env
            deploy_direct
            ;;
        update)
            update_project
            ;;
        install)
            install_global_command
            ;;
        stop)
            stop_service
            ;;
        status)
            show_status
            ;;
        uninstall)
            uninstall_service
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
            echo "6) 更新项目"
            echo "7) 安装全局命令"
            echo "8) 卸载服务"
            echo "9) 退出"
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

            read -p "请输入您的选择 [1-9]: " choice

            case $choice in
                1)
                    # 如果不在项目目录，先克隆
                    if ! check_in_project_dir; then
                        check_prerequisites git true
                        clone_repository
                    fi
                    check_prerequisites docker true
                    setup_env
                    deploy_docker
                    ;;
                2)
                    # 如果不在项目目录，先克隆
                    if ! check_in_project_dir; then
                        check_prerequisites git true
                        clone_repository
                    fi
                    check_prerequisites docker true
                    setup_env
                    deploy_docker_compose
                    ;;
                3)
                    # 如果不在项目目录，先克隆
                    if ! check_in_project_dir; then
                        check_prerequisites git true
                        clone_repository
                    fi
                    check_prerequisites go true
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
                    update_project
                    ;;
                7)
                    install_global_command
                    ;;
                8)
                    uninstall_service
                    ;;
                9)
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
