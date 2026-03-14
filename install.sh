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
TMP_DIR=""

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

cleanup() {
  if [ -n "${TMP_DIR:-}" ] && [ -d "$TMP_DIR" ]; then
    rm -rf "$TMP_DIR"
  fi
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

extract_port() {
  local address="$1"
  if [[ "$address" =~ :([0-9]+)$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
    return 0
  fi
  return 1
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
  local input_fd=""
  if [ -r /dev/tty ]; then
    input_fd="/dev/tty"
  fi
  if [ "$secret" = "true" ]; then
    if [ -n "$input_fd" ]; then
      read -r -s -p "$prompt" input <"$input_fd"
      printf '\n' >"$input_fd"
    else
      read -r -s -p "$prompt" input
      echo ""
    fi
  else
    if [ -n "$input_fd" ]; then
      read -r -p "$prompt" input <"$input_fd"
    else
      read -r -p "$prompt" input
    fi
  fi
  input="${input//$'\r'/}"
  printf '%s' "$input"
}

should_keep_existing_config() {
  if [ ! -f "$CONFIG_PATH" ]; then
    return 1
  fi

  local keep_config
  read -r -p "检测到已有配置文件，是否保留现有配置? [Y/n]: " keep_config
  keep_config="${keep_config:-Y}"
  if [[ "$keep_config" =~ ^[Yy]$ ]]; then
    return 0
  fi
  return 1
}

read_config_api_key() {
  if [ ! -f "$CONFIG_PATH" ]; then
    return 1
  fi
  sed -n 's/^[[:space:]]*apiKey:[[:space:]]*"\{0,1\}\([^"]*\)"\{0,1\}[[:space:]]*$/\1/p' "$CONFIG_PATH" | head -n 1
}

upsert_config_api_key() {
  local api_key="$1"
  local escaped_key tmp_config
  escaped_key="$(escape_yaml "$api_key")"
  tmp_config="$(mktemp)"

  awk -v new_line="apiKey: \"${escaped_key}\"" '
    BEGIN { replaced = 0 }
    /^[[:space:]]*apiKey:[[:space:]]*/ && replaced == 0 {
      print new_line
      replaced = 1
      next
    }
    { print }
    END {
      if (replaced == 0) {
        print new_line
      }
    }
  ' "$CONFIG_PATH" >"$tmp_config"

  as_root install -m 0644 "$tmp_config" "$CONFIG_PATH"
  rm -f "$tmp_config"
}

write_config() {
  local api_key="$1"
  local address="$2"
  local proxy="$3"
  local force_ip_family="$4"

  local tmp_config
  tmp_config="$(mktemp)"

  {
    echo "sessions: []"
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

verify_service() {
  local address="$1"
  local api_key="$2"

  if ! as_root systemctl is-active --quiet "$SERVICE_NAME"; then
    error "systemd 服务未处于运行状态，请执行 systemctl status ${SERVICE_NAME} 查看详情"
    exit 1
  fi

  local port
  port="$(extract_port "$address" || true)"
  if [ -z "$port" ]; then
    warn "未能从 address=${address} 中提取端口，跳过本地 HTTP 健康检查"
    return 0
  fi

  if curl -fsS --max-time 5 -H "Authorization: Bearer ${api_key}" "http://127.0.0.1:${port}/health" >/dev/null; then
    success "本地健康检查通过: http://127.0.0.1:${port}/health"
    return 0
  fi

  warn "服务已启动，但本地健康检查未通过，请执行 journalctl -u ${SERVICE_NAME} -f 查看日志"
}

main() {
  if [ "$(uname -s)" != "Linux" ]; then
    error "一键部署脚本目前仅支持 Linux"
    exit 1
  fi

  install_pkg_if_missing curl
  install_pkg_if_missing tar

  local arch tag
  arch="$(detect_arch)"
  tag="${INSTALL_TAG:-$(fetch_latest_tag)}"
  if [ -z "$tag" ]; then
    error "未获取到最新 Release，请先确认 Releases 页面已有正式版本"
    exit 1
  fi

  TMP_DIR="$(mktemp -d)"
  trap cleanup EXIT

  local api_key address proxy force_ip_family config_kept existing_api_key
  config_kept="false"
  address="${ADDRESS:-0.0.0.0:8080}"
  proxy="${PROXY:-}"
  force_ip_family="${FORCE_IP_FAMILY:-auto}"

  if should_keep_existing_config; then
    config_kept="true"
    info "保留现有配置: ${CONFIG_PATH}"
    existing_api_key="$(read_config_api_key || true)"
    if [ -n "$existing_api_key" ]; then
      api_key="$existing_api_key"
    else
      warn "现有配置中的 APIKEY 为空或格式异常，将修正这一项"
      api_key="$(prompt_if_empty "${APIKEY:-}" "请输入 APIKEY（输入时不回显）: " true)"
      upsert_config_api_key "$api_key"
      success "已修正现有配置中的 APIKEY: ${CONFIG_PATH}"
    fi
  else
    api_key="$(prompt_if_empty "${APIKEY:-}" "请输入 APIKEY（输入时不回显）: " true)"
  fi

  if [ -z "$api_key" ]; then
    error "APIKEY 不能为空"
    exit 1
  fi

  download_release "$tag" "$arch" "$TMP_DIR"
  install_files "$TMP_DIR"
  if [ "$config_kept" != "true" ]; then
    write_config "$api_key" "$address" "$proxy" "$force_ip_family"
  fi
  setup_service
  verify_service "$address" "$api_key"

  if [ "$config_kept" != "true" ]; then
    warn "当前未写入任何账号 Cookie，请先访问 /admin/setup 设置后台密码，再到 /admin 添加账号"
  fi

  echo ""
  success "部署完成"
  info "当前版本: ${tag}"
  info "服务状态: systemctl status ${SERVICE_NAME}"
  info "服务日志: journalctl -u ${SERVICE_NAME} -f"
  info "管理后台: http://<服务器IP>:$(extract_port "$address" || printf '8080')/admin"
  info "健康检查: curl -H 'Authorization: Bearer <APIKEY>' http://127.0.0.1:$(extract_port "$address" || printf '8080')/health"
  info "模型列表: curl -H 'Authorization: Bearer <APIKEY>' http://127.0.0.1:$(extract_port "$address" || printf '8080')/v1/models"
}

main "$@"
