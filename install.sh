#!/usr/bin/env bash

set -euo pipefail

APP_NAME="c2c"
REPO="wosa1402/claude2api"
SERVICE_NAME="c2c"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
CONFIG_DIR="${CONFIG_DIR:-/etc/c2c}"
BINARY_PATH="${INSTALL_DIR}/${APP_NAME}"
CONFIG_PATH="${CONFIG_DIR}/config.yaml"
EXAMPLE_CONFIG_PATH="${CONFIG_DIR}/config.yaml.example"
SERVICE_PATH="/etc/systemd/system/${SERVICE_NAME}.service"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

info() {
  echo -e "${BLUE}[信息]${NC} $1"
}

success() {
  echo -e "${GREEN}[成功]${NC} $1"
}

warn() {
  echo -e "${YELLOW}[提示]${NC} $1"
}

error() {
  echo -e "${RED}[错误]${NC} $1" >&2
}

command_exists() {
  command -v "$1" >/dev/null 2>&1
}

as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif command_exists sudo; then
    sudo "$@"
  else
    error "需要 root 或 sudo 权限"
    exit 1
  fi
}

install_pkg_if_missing() {
  local pkg="$1"
  if command_exists "$pkg"; then
    return 0
  fi

  warn "未检测到 ${pkg}，尝试自动安装"
  if command_exists apt-get; then
    as_root apt-get update
    as_root apt-get install -y "$pkg"
  elif command_exists dnf; then
    as_root dnf install -y "$pkg"
  elif command_exists yum; then
    as_root yum install -y "$pkg"
  else
    error "无法自动安装 ${pkg}，请先手动安装"
    exit 1
  fi
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64)
      echo "amd64"
      ;;
    aarch64|arm64)
      echo "arm64"
      ;;
    *)
      error "当前架构不支持: $(uname -m)"
      exit 1
      ;;
  esac
}

escape_yaml() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

fetch_latest_tag() {
  local response
  response="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest")"
  printf '%s\n' "$response" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

download_release() {
  local tag="$1"
  local arch="$2"
  local tmp_dir="$3"
  local asset="${APP_NAME}_${tag}_linux_${arch}.tar.gz"
  local url="https://github.com/${REPO}/releases/download/${tag}/${asset}"

  info "正在下载 ${asset}"
  curl -fL "$url" -o "${tmp_dir}/${asset}"

  mkdir -p "${tmp_dir}/extract"
  tar -xzf "${tmp_dir}/${asset}" -C "${tmp_dir}/extract"

  if [ ! -f "${tmp_dir}/extract/${APP_NAME}" ]; then
    error "压缩包中未找到 ${APP_NAME} 可执行文件"
    exit 1
  fi
}

prompt_if_empty() {
  local value="$1"
  local prompt="$2"
  local secret="${3:-false}"
  if [ -n "$value" ]; then
    printf '%s' "$value"
    return 0
  fi

  local input=""
  if [ "$secret" = "true" ]; then
    read -r -s -p "$prompt" input
    echo ""
  else
    read -r -p "$prompt" input
  fi
  printf '%s' "$input"
}

write_config() {
  local sessions="$1"
  local api_key="$2"
  local address="$3"
  local proxy="$4"
  local force_ip_family="$5"

  if [ -f "$CONFIG_PATH" ]; then
    read -r -p "检测到已有配置文件，是否保留现有配置? [Y/n]: " keep_config
    keep_config="${keep_config:-Y}"
    if [[ "$keep_config" =~ ^[Yy]$ ]]; then
      info "保留现有配置: ${CONFIG_PATH}"
      return 0
    fi
  fi

  local tmp_config
  tmp_config="$(mktemp)"

  {
    echo "sessions:"
    IFS=',' read -r -a session_items <<< "$sessions"
    for raw_item in "${session_items[@]}"; do
      local item trimmed key org
      item="$(printf '%s' "$raw_item" | xargs)"
      [ -n "$item" ] || continue
      key="${item%%:*}"
      org=""
      if [ "$item" != "$key" ]; then
        org="${item#*:}"
      fi
      trimmed="$(escape_yaml "$key")"
      org="$(escape_yaml "$org")"
      echo "  - sessionKey: \"${trimmed}\""
      echo "    orgID: \"${org}\""
      echo "    enabled: true"
      echo "    pool: \"low\""
    done
    echo ""
    echo "address: \"$(escape_yaml "$address")\""
    echo "apiKey: \"$(escape_yaml "$api_key")\""
    echo "proxy: \"$(escape_yaml "$proxy")\""
    echo "forceIPFamily: \"$(escape_yaml "$force_ip_family")\""
    echo "chatDelete: true"
    echo "maxChatHistoryLength: 10000"
    echo "noRolePrefix: false"
    echo "promptDisableArtifacts: false"
    echo "enableMirrorApi: false"
    echo "mirrorApiPrefix: \"\""
  } >"$tmp_config"

  as_root install -d "$CONFIG_DIR"
  as_root install -m 0644 "$tmp_config" "$CONFIG_PATH"
  rm -f "$tmp_config"
  success "配置文件已生成: ${CONFIG_PATH}"
}

install_files() {
  local tmp_dir="$1"

  as_root install -d "$INSTALL_DIR" "$CONFIG_DIR"
  as_root install -m 0755 "${tmp_dir}/extract/${APP_NAME}" "$BINARY_PATH"
  if [ -f "${tmp_dir}/extract/config.yaml.example" ]; then
    as_root install -m 0644 "${tmp_dir}/extract/config.yaml.example" "$EXAMPLE_CONFIG_PATH"
  fi
}

setup_service() {
  if ! command_exists systemctl; then
    error "未检测到 systemd，当前脚本暂时只支持 systemd Linux"
    exit 1
  fi

  local tmp_service
  tmp_service="$(mktemp)"
  cat >"$tmp_service" <<EOF
[Unit]
Description=c2c service
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${CONFIG_DIR}
ExecStart=${BINARY_PATH}
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

  as_root install -m 0644 "$tmp_service" "$SERVICE_PATH"
  rm -f "$tmp_service"

  as_root systemctl daemon-reload
  as_root systemctl enable --now "$SERVICE_NAME"
  success "systemd 服务已启动: ${SERVICE_NAME}"
}

main() {
  if [ "$(uname -s)" != "Linux" ]; then
    error "一键部署脚本目前仅支持 Linux"
    exit 1
  fi

  install_pkg_if_missing curl
  install_pkg_if_missing tar

  local arch tag tmp_dir
  arch="$(detect_arch)"
  tag="${INSTALL_TAG:-$(fetch_latest_tag)}"
  if [ -z "$tag" ]; then
    error "未获取到最新 Release，请先确认 Releases 页面已有正式版本"
    exit 1
  fi

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  local sessions api_key address proxy force_ip_family
  sessions="$(prompt_if_empty "${SESSIONS:-}" "请输入 SESSIONS（多个账号用逗号分隔）: ")"
  api_key="$(prompt_if_empty "${APIKEY:-}" "请输入 APIKEY: " true)"
  address="${ADDRESS:-0.0.0.0:8080}"
  proxy="${PROXY:-}"
  force_ip_family="${FORCE_IP_FAMILY:-auto}"

  if [ -z "$sessions" ] || [ -z "$api_key" ]; then
    error "SESSIONS 和 APIKEY 不能为空"
    exit 1
  fi

  download_release "$tag" "$arch" "$tmp_dir"
  install_files "$tmp_dir"
  write_config "$sessions" "$api_key" "$address" "$proxy" "$force_ip_family"
  setup_service

  echo ""
  success "部署完成"
  info "当前版本: ${tag}"
  info "服务状态: systemctl status ${SERVICE_NAME}"
  info "服务日志: journalctl -u ${SERVICE_NAME} -f"
  info "访问地址: http://${address}"
}

main "$@"
