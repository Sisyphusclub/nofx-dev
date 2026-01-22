#!/bin/bash
set -e

# NOFX VPS 一键部署脚本
# 用法: curl -fsSL <url> | bash
# 或者: bash deploy-vps.sh

REPO_URL="https://github.com/Sisyphusclub/nofx-dev.git"
BRANCH="dev"
INSTALL_DIR="${NOFX_INSTALL_DIR:-$HOME/nofx-dev}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[OK]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; exit 1; }

echo ""
echo "=========================================="
echo "     NOFX Trading System Deployment"
echo "=========================================="
echo ""

# 检查依赖
check_dependencies() {
    log_info "检查系统依赖..."

    if ! command -v git &> /dev/null; then
        log_error "git 未安装。请先安装: apt install git 或 yum install git"
    fi
    log_success "git 已安装"

    if ! command -v docker &> /dev/null; then
        log_error "docker 未安装。请先安装: https://docs.docker.com/engine/install/"
    fi
    log_success "docker 已安装"

    if ! docker compose version &> /dev/null && ! docker-compose version &> /dev/null; then
        log_error "docker compose 未安装。请安装 Docker Compose V2"
    fi
    log_success "docker compose 已安装"

    if ! command -v openssl &> /dev/null; then
        log_error "openssl 未安装。请先安装: apt install openssl"
    fi
    log_success "openssl 已安装"
}

# 克隆或更新项目
clone_or_update_repo() {
    if [ -d "$INSTALL_DIR" ]; then
        log_info "项目目录已存在，更新代码..."
        cd "$INSTALL_DIR"
        git fetch origin
        git checkout "$BRANCH"
        git pull origin "$BRANCH"
        log_success "代码已更新"
    else
        log_info "克隆项目到 $INSTALL_DIR ..."
        git clone -b "$BRANCH" "$REPO_URL" "$INSTALL_DIR"
        cd "$INSTALL_DIR"
        log_success "项目已克隆"
    fi
}

# 生成 .env 文件
generate_env() {
    if [ -f "$INSTALL_DIR/.env" ]; then
        log_warn ".env 文件已存在，跳过生成（如需重新生成请先删除）"
        return
    fi

    log_info "生成安全密钥..."

    JWT_SECRET=$(openssl rand -base64 32)
    DATA_KEY=$(openssl rand -base64 32)
    RSA_KEY=$(openssl genrsa 2048 2>/dev/null | sed ':a;N;$!ba;s/\n/\\n/g')

    log_info "创建 .env 配置文件..."

    cat > "$INSTALL_DIR/.env" << EOF
# NOFX Environment Configuration
# Generated at $(date)

# ===========================================
# Server Configuration
# ===========================================
NOFX_BACKEND_PORT=8080
NOFX_FRONTEND_PORT=3000
NOFX_TIMEZONE=Asia/Shanghai

# ===========================================
# Authentication (Auto-generated)
# ===========================================
JWT_SECRET=${JWT_SECRET}

# ===========================================
# Encryption Keys (Auto-generated)
# ===========================================
DATA_ENCRYPTION_KEY=${DATA_KEY}
RSA_PRIVATE_KEY=${RSA_KEY}

# ===========================================
# Security Options
# ===========================================
TRANSPORT_ENCRYPTION=false

# ===========================================
# Database Configuration
# ===========================================
DB_TYPE=sqlite
DB_PATH=data/data.db

# ===========================================
# Optional: Telegram Notifications
# ===========================================
# TELEGRAM_BOT_TOKEN=your-bot-token
EOF

    chmod 600 "$INSTALL_DIR/.env"
    log_success ".env 配置文件已生成"
}

# 创建数据目录
create_data_dir() {
    log_info "创建数据目录..."
    mkdir -p "$INSTALL_DIR/data"
    log_success "数据目录已创建: $INSTALL_DIR/data"
}

# 构建并启动容器
start_containers() {
    cd "$INSTALL_DIR"

    log_info "停止旧容器（如有）..."
    docker compose down 2>/dev/null || true

    log_info "构建 Docker 镜像（首次可能需要几分钟）..."
    docker compose build --no-cache

    log_info "启动容器..."
    docker compose up -d

    log_success "容器已启动"
}

# 等待服务就绪
wait_for_health() {
    log_info "等待服务启动..."

    local max_attempts=30
    local attempt=0

    while [ $attempt -lt $max_attempts ]; do
        if curl -s http://localhost:8080/api/health > /dev/null 2>&1; then
            log_success "后端服务已就绪"
            break
        fi
        attempt=$((attempt + 1))
        echo -n "."
        sleep 2
    done
    echo ""

    if [ $attempt -eq $max_attempts ]; then
        log_warn "服务启动超时，请检查日志: docker compose logs"
    fi
}

# 显示结果
show_result() {
    local ip=$(curl -s ifconfig.me 2>/dev/null || echo "YOUR_VPS_IP")

    echo ""
    echo "=========================================="
    echo -e "${GREEN}    部署完成！${NC}"
    echo "=========================================="
    echo ""
    echo "访问地址:"
    echo -e "  前端: ${BLUE}http://${ip}:3000${NC}"
    echo -e "  后端: ${BLUE}http://${ip}:8080${NC}"
    echo ""
    echo "常用命令:"
    echo "  cd $INSTALL_DIR"
    echo "  docker compose logs -f      # 查看日志"
    echo "  docker compose restart      # 重启服务"
    echo "  docker compose down         # 停止服务"
    echo "  docker compose up -d        # 启动服务"
    echo ""
    echo "配置文件: $INSTALL_DIR/.env"
    echo "数据目录: $INSTALL_DIR/data"
    echo ""
}

# 主流程
main() {
    check_dependencies
    clone_or_update_repo
    generate_env
    create_data_dir
    start_containers
    wait_for_health
    show_result
}

main "$@"
