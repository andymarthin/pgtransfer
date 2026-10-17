#!/bin/sh
# pgtransfer installer for Linux and macOS.
#
# Downloads the prebuilt pgtransfer binary for this machine, verifies it against
# the release's SHA256SUMS, and installs it.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/andymarthin/pgtransfer/main/scripts/install.sh | sh
#
# Options (via environment variables):
#   PGTRANSFER_VERSION   Install a specific tag, e.g. v1.1.0 (default: latest)
#   PGTRANSFER_INSTALL   Install directory (default: /usr/local/bin if writable,
#                        otherwise ~/.local/bin)
#   PGTRANSFER_NO_SUDO   Set to 1 to never use sudo (fail instead of prompting)
#
# Examples:
#   # specific version, custom location
#   curl -fsSL .../install.sh | PGTRANSFER_VERSION=v1.1.0 PGTRANSFER_INSTALL="$HOME/bin" sh
#
#   # no sudo prompt; falls back to ~/.local/bin
#   curl -fsSL .../install.sh | PGTRANSFER_NO_SUDO=1 sh

set -eu

REPO="andymarthin/pgtransfer"
BIN="pgtransfer"
PROG="pgtransfer-install"

# --- pretty output (only when stderr is a terminal) -----------------------
if [ -t 2 ]; then
    C_RESET=$(printf '\033[0m')
    C_INFO=$(printf '\033[36m')
    C_OK=$(printf '\033[32m')
    C_WARN=$(printf '\033[33m')
    C_ERR=$(printf '\033[31m')
else
    C_RESET='' C_INFO='' C_OK='' C_WARN='' C_ERR=''
fi

info() { printf '%s==>%s %s\n' "$C_INFO" "$C_RESET" "$*" >&2; }
ok()   { printf '%s==>%s %s\n' "$C_OK" "$C_RESET" "$*" >&2; }
warn() { printf '%s==>%s %s\n' "$C_WARN" "$C_RESET" "$*" >&2; }
die()  { printf '%s==> error:%s %s\n' "$C_ERR" "$C_RESET" "$*" >&2; exit 1; }

need() {
    command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"
}

# --- prerequisites ---------------------------------------------------------
need uname
need tar

# A downloader is required; prefer curl, fall back to wget.
if command -v curl >/dev/null 2>&1; then
    fetch() { curl -fsSL "$1"; }
    download() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
    fetch() { wget -qO- "$1"; }
    download() { wget -qO "$2" "$1"; }
else
    die "either curl or wget is required"
fi

# A checksum tool is required to verify the download.
if command -v sha256sum >/dev/null 2>&1; then
    sha256_of() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
    sha256_of() { shasum -a 256 "$1" | awk '{print $1}'; }
elif command -v openssl >/dev/null 2>&1; then
    sha256_of() { openssl dgst -sha256 "$1" | awk '{print $NF}'; }
else
    die "no SHA-256 tool found (need sha256sum, shasum, or openssl)"
fi

# --- detect platform -------------------------------------------------------
os=$(uname -s)
case "$os" in
    Linux)  os_name=linux ;;
    Darwin) os_name=darwin ;;
    *) die "unsupported operating system: $os (use the release archives, or build from source)" ;;
esac

machine=$(uname -m)
case "$machine" in
    x86_64|amd64)   arch=amd64 ;;
    aarch64|arm64)  arch=arm64 ;;
    *) die "unsupported architecture: $machine" ;;
esac

# --- resolve version -------------------------------------------------------
version=${PGTRANSFER_VERSION:-}
if [ -z "$version" ]; then
    info "Resolving the latest release..."
    # Prefer the GitHub API; fall back to following the /releases/latest redirect
    # so the installer still works where the API is rate-limited or blocked.
    version=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
        | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' \
        | head -n1) || true

    if [ -z "$version" ]; then
        warn "GitHub API unavailable; falling back to the redirect lookup"
        version=$(fetch -o /dev/null -w '%{url_effective}' "https://github.com/${REPO}/releases/latest" 2>/dev/null \
            | sed 's#.*/tag/##') || true
    fi
fi
[ -n "$version" ] || die "could not determine the latest version; set PGTRANSFER_VERSION=vX.Y.Z"
case "$version" in
    v*) ;;
    *) version="v$version" ;;
esac

asset="${BIN}-${version}-${os_name}-${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${version}"

info "Installing ${BIN} ${version} (${os_name}/${arch})"

# --- install directory -----------------------------------------------------
install_dir=${PGTRANSFER_INSTALL:-}
use_sudo=0
if [ -z "$install_dir" ]; then
    if [ -w /usr/local/bin ] 2>/dev/null; then
        install_dir=/usr/local/bin
    else
        install_dir="$HOME/.local/bin"
    fi
fi

if [ ! -w "$install_dir" ]; then
    # Directory exists but is not writable, or does not exist yet.
    if [ -d "$install_dir" ]; then
        if [ "${PGTRANSFER_NO_SUDO:-0}" = "1" ]; then
            die "$install_dir is not writable; re-run with PGTRANSFER_INSTALL=\"\$HOME/.local/bin\""
        fi
        if command -v sudo >/dev/null 2>&1; then
            use_sudo=1
            info "$install_dir is not writable; sudo is required"
        else
            die "$install_dir is not writable and sudo is unavailable"
        fi
    else
        mkdir -p "$install_dir" || die "could not create $install_dir"
    fi
fi

# --- download and verify ---------------------------------------------------
tmp=$(mktemp -d 2>/dev/null || mktemp -d -t pgtransfer)
# shellcheck disable=SC2064
trap "rm -rf '$tmp'" EXIT INT TERM

info "Downloading ${asset}"
download "${base}/${asset}" "${tmp}/${asset}" || die "download failed: ${base}/${asset}"

info "Downloading checksums"
download "${base}/SHA256SUMS" "${tmp}/SHA256SUMS" || die "could not download SHA256SUMS"

expected=$(awk -v a="$asset" '$2 == a {print $1}' "${tmp}/SHA256SUMS")
[ -n "$expected" ] || die "no checksum published for ${asset}"

actual=$(sha256_of "${tmp}/${asset}")
if [ "$expected" != "$actual" ]; then
    die "checksum mismatch for ${asset}
  expected: ${expected}
  actual:   ${actual}"
fi
ok "Checksum verified"

# --- install ---------------------------------------------------------------
info "Extracting"
tar -xzf "${tmp}/${asset}" -C "$tmp" || die "failed to extract ${asset}"
[ -f "${tmp}/${BIN}" ] || die "archive did not contain a ${BIN} binary"

if [ "$use_sudo" = "1" ]; then
    sudo install -m 0755 "${tmp}/${BIN}" "${install_dir}/${BIN}"
else
    install -m 0755 "${tmp}/${BIN}" "${install_dir}/${BIN}" 2>/dev/null \
        || { cp "${tmp}/${BIN}" "${install_dir}/${BIN}" && chmod 0755 "${install_dir}/${BIN}"; }
fi

ok "Installed ${BIN} to ${install_dir}/${BIN}"

# --- post-install guidance -------------------------------------------------
case ":$PATH:" in
    *":${install_dir}:"*) ;;
    *)
        warn "${install_dir} is not on your PATH. Add it with:"
        printf '\n    export PATH="%s:$PATH"\n\n' "$install_dir" >&2
        ;;
esac

if installed_version=$("${install_dir}/${BIN}" version 2>/dev/null | head -n1); then
    ok "$installed_version"
fi

# --- optional dependency check ---------------------------------------------
# pgtransfer itself is installed. These tools are only needed for some commands,
# so they are reported as guidance rather than treated as install failures.

missing_pg=""
for tool in pg_dump pg_restore psql; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        missing_pg="${missing_pg}${missing_pg:+ }${tool}"
    fi
done

# The SSH tunnel runs in-process (golang.org/x/crypto/ssh), so no ssh binary is
# required. What a tunnelled profile needs is a way to authenticate: an agent, a
# key file or a password. Only the first two can be seen from here.
missing_ssh=""
if [ -z "${SSH_AUTH_SOCK:-}" ] && [ ! -f "${HOME}/.ssh/id_ed25519" ] && [ ! -f "${HOME}/.ssh/id_rsa" ]; then
    missing_ssh="no-agent-or-default-key"
fi

# Detect the package manager so the advice is copy-pasteable.
pkg_hint() {
    if [ "$os_name" = "darwin" ]; then
        echo "brew install postgresql@17"
        return
    fi
    if command -v apt-get >/dev/null 2>&1; then
        echo "sudo apt-get update && sudo apt-get install -y postgresql-client"
    elif command -v dnf >/dev/null 2>&1; then
        echo "sudo dnf install -y postgresql"
    elif command -v yum >/dev/null 2>&1; then
        echo "sudo yum install -y postgresql"
    elif command -v apk >/dev/null 2>&1; then
        echo "sudo apk add postgresql-client"
    elif command -v pacman >/dev/null 2>&1; then
        echo "sudo pacman -S postgresql"
    elif command -v zypper >/dev/null 2>&1; then
        echo "sudo zypper install postgresql-client"
    else
        echo "install the PostgreSQL client tools with your package manager"
    fi
}

ssh_hint() {
    echo "start an ssh-agent and run ssh-add, or pass --ssh-key when adding a profile"
}

if [ -z "$missing_pg" ] && [ -z "$missing_ssh" ]; then
    ok "PostgreSQL client tools are available"
else
    printf '\n' >&2
    if [ -n "$missing_pg" ]; then
        warn "Missing PostgreSQL client tool(s): ${missing_pg}"
        cat >&2 <<EOF
    Needed for: pgtransfer export dump / import dump / migrate
    Install with:
        $(pkg_hint)
EOF
    fi
    if [ -n "$missing_ssh" ]; then
        warn "No SSH agent running and no default key found"
        cat >&2 <<EOF
    Needed for: profiles with SSH tunnels enabled, which have to authenticate
    to the bastion. Either start an agent and add a key, or give each tunnelled
    profile its own key or password:
        $(ssh_hint)
EOF
    fi
    printf '\n%s  This is only required for those commands; pgtransfer itself is installed and ready.%s\n' "$C_INFO" "$C_RESET" >&2
fi

cat >&2 <<'EOF'

Next steps:
  pgtransfer profile add <name>        # create a connection profile
  pgtransfer test-connection <name>    # verify the connection
  pgtransfer --json version            # machine-readable output for scripts
  pgtransfer mcp                       # run as an MCP server for AI agents
EOF
