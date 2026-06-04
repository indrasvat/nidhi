#!/bin/sh
set -eu

# nidhi installer
# Usage:
#   curl -sSfL https://raw.githubusercontent.com/indrasvat/nidhi/main/install.sh | sh
#   curl -sSfL https://raw.githubusercontent.com/indrasvat/nidhi/main/install.sh | sh -s -- --version v0.1.0 --dir "$HOME/.local/bin"

REPO="indrasvat/nidhi"
BINARY="nidhi"
DEFAULT_DIR="${NIDHI_INSTALL_DIR:-${HOME}/.local/bin}"
API_BASE_URL="${NIDHI_INSTALL_API_BASE_URL:-https://api.github.com/repos/${REPO}}"
RELEASE_BASE_URL="${NIDHI_INSTALL_RELEASE_BASE_URL:-https://github.com/${REPO}/releases/download}"

setup_colors() {
    if [ -n "${NO_COLOR:-}" ] || [ ! -t 1 ]; then
        BOLD=""
        RESET=""
        GOLD=""
        BRIGHT=""
        AQUA=""
        BLUE=""
        PURPLE=""
        GREEN=""
        RED=""
        YELLOW=""
        BG_DEEP=""
        BG_SURFACE=""
        TEXT=""
        SUBTEXT=""
        DIM=""
    else
        BOLD="$(printf '\033[1m')"
        RESET="$(printf '\033[0m')"
        GOLD="$(printf '\033[38;2;212;160;80m')"
        BRIGHT="$(printf '\033[38;2;232;184;90m')"
        AQUA="$(printf '\033[38;2;78;201;176m')"
        BLUE="$(printf '\033[38;2;97;175;239m')"
        PURPLE="$(printf '\033[38;2;198;120;221m')"
        GREEN="$(printf '\033[38;2;115;217;144m')"
        RED="$(printf '\033[38;2;255;95;109m')"
        YELLOW="$(printf '\033[38;2;229;192;123m')"
        BG_DEEP="$(printf '\033[48;2;7;9;14m')"
        BG_SURFACE="$(printf '\033[48;2;15;18;25m')"
        TEXT="$(printf '\033[38;2;200;204;212m')"
        SUBTEXT="$(printf '\033[38;2;107;114;128m')"
        DIM="$(printf '\033[38;2;61;68;80m')"
    fi
}

banner() {
    cols="$(tput cols 2>/dev/null || printf '80')"
    if [ "${cols}" -lt 70 ]; then
        return
    fi

    printf '\n'
    printf '%s%s███╗   ██╗██╗██████╗ ██╗  ██╗██╗%s\n' "${BG_DEEP}" "${BRIGHT}" "${RESET}"
    printf '%s%s████╗  ██║██║██╔══██╗██║  ██║██║%s\n' "${BG_DEEP}" "${BRIGHT}" "${RESET}"
    printf '%s%s██╔██╗ ██║██║██║  ██║███████║██║%s\n' "${BG_DEEP}" "${GOLD}" "${RESET}"
    printf '%s%s██║╚██╗██║██║██║  ██║██╔══██║██║%s\n' "${BG_DEEP}" "${GOLD}" "${RESET}"
    printf '%s%s██║ ╚████║██║██████╔╝██║  ██║██║%s\n' "${BG_DEEP}" "${YELLOW}" "${RESET}"
    printf '%s%s╚═╝  ╚═══╝╚═╝╚═════╝ ╚═╝  ╚═╝╚═╝%s\n' "${BG_DEEP}" "${DIM}" "${RESET}"
    printf '%s%s  purpose-built TUI for git stash mastery%s\n' "${BG_DEEP}" "${SUBTEXT}" "${RESET}"
    printf '%s%s  Agni installer · semver GitHub releases%s\n\n' "${BG_DEEP}" "${AQUA}" "${RESET}"
}

info() {
    printf '  %s◆%s %s%s%s\n' "${BLUE}" "${RESET}" "${TEXT}" "$1" "${RESET}"
}

success() {
    printf '  %s✓%s %s%s%s\n' "${GREEN}" "${RESET}" "${TEXT}" "$1" "${RESET}"
}

warn() {
    printf '  %s! %s%s\n' "${YELLOW}" "$1" "${RESET}"
}

error_exit() {
    printf '  %s✗%s %s%s%s\n' "${RED}" "${RESET}" "${TEXT}" "$1" "${RESET}" >&2
    exit 1
}

step() {
    printf '\n%s%s %s/%s %s%s%s %s%s%s\n' "${BG_SURFACE}" "${GOLD}" "$1" "$2" "${RESET}" "${BOLD}" "${TEXT}" "$3" "${SUBTEXT}" "${RESET}"
}

shorten() {
    text="$1"
    max="$2"
    if [ "${#text}" -le "${max}" ]; then
        printf '%s' "${text}"
        return
    fi

    keep=$((max - 3))
    printf '%s...' "$(printf '%s' "${text}" | cut -c 1-"${keep}")"
}

release_panel() {
    release_value="$(shorten "${VERSION}" 45)"
    artifact_value="$(shorten "${TARBALL}" 45)"
    install_value="$(shorten "${INSTALL_DIR}" 45)"

    printf '\n'
    printf '%s╭────────────────────────────────────────────────────────╮%s\n' "${DIM}" "${RESET}"
    printf '%s│%s %srelease %s  %-45s%s│%s\n' "${DIM}" "${RESET}" "${GOLD}" "${RESET}" "${release_value}" "${DIM}" "${RESET}"
    printf '%s│%s %sartifact%s  %-45s%s│%s\n' "${DIM}" "${RESET}" "${PURPLE}" "${RESET}" "${artifact_value}" "${DIM}" "${RESET}"
    printf '%s│%s %sinstall %s  %-45s%s│%s\n' "${DIM}" "${RESET}" "${AQUA}" "${RESET}" "${install_value}" "${DIM}" "${RESET}"
    printf '%s╰────────────────────────────────────────────────────────╯%s\n' "${DIM}" "${RESET}"
}

usage() {
    printf '%s%snidhi installer%s\n\n' "${BOLD}" "${TEXT}" "${RESET}"
    printf '%sUsage:%s\n' "${SUBTEXT}" "${RESET}"
    printf '  curl -sSfL https://raw.githubusercontent.com/%s/main/install.sh | sh\n' "${REPO}"
    printf '  curl -sSfL https://raw.githubusercontent.com/%s/main/install.sh | sh -s -- [OPTIONS]\n\n' "${REPO}"
    printf '%sOptions:%s\n' "${SUBTEXT}" "${RESET}"
    printf '  %s--version VERSION%s  Install a specific SemVer tag, e.g. v0.1.0\n' "${TEXT}" "${RESET}"
    printf '  %s--dir DIRECTORY%s    Install directory (default: %s)\n' "${TEXT}" "${RESET}" "${DEFAULT_DIR}"
    printf '  %s--help%s             Show this help\n' "${TEXT}" "${RESET}"
    exit 0
}

normalize_version() {
    case "${VERSION}" in
        "") return ;;
        v*) ;;
        *) VERSION="v${VERSION}" ;;
    esac

    if ! printf '%s\n' "${VERSION}" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+([-.][0-9A-Za-z.-]+)?$'; then
        error_exit "Version must be a SemVer tag like v0.1.0; got ${VERSION}"
    fi
}

parse_args() {
    VERSION=""
    INSTALL_DIR="${DEFAULT_DIR}"

    while [ "$#" -gt 0 ]; do
        case "$1" in
            --version)
                [ "$#" -ge 2 ] || error_exit "--version requires a value"
                VERSION="$2"
                shift 2
                ;;
            --dir)
                [ "$#" -ge 2 ] || error_exit "--dir requires a value"
                INSTALL_DIR="$2"
                shift 2
                ;;
            --help|-h)
                usage
                ;;
            *)
                error_exit "Unknown option: $1 (use --help for usage)"
                ;;
        esac
    done

    normalize_version
}

check_dependencies() {
    if command -v curl >/dev/null 2>&1; then
        DOWNLOADER="curl"
        success "Using curl for downloads"
    elif command -v wget >/dev/null 2>&1; then
        DOWNLOADER="wget"
        success "Using wget for downloads"
    else
        error_exit "curl or wget is required"
    fi

    if command -v shasum >/dev/null 2>&1; then
        HASHER="shasum"
        success "Using shasum for checksum verification"
    elif command -v sha256sum >/dev/null 2>&1; then
        HASHER="sha256sum"
        success "Using sha256sum for checksum verification"
    else
        error_exit "shasum or sha256sum is required"
    fi

    command -v tar >/dev/null 2>&1 || error_exit "tar is required"
    command -v mktemp >/dev/null 2>&1 || error_exit "mktemp is required"
}

detect_platform() {
    os="$(uname -s)"
    case "${os}" in
        Darwin) OS="darwin" ;;
        Linux) OS="linux" ;;
        *) error_exit "Unsupported operating system: ${os}" ;;
    esac

    arch="$(uname -m)"
    case "${arch}" in
        arm64|aarch64) ARCH="arm64" ;;
        x86_64|amd64)
            if [ "${OS}" = "darwin" ]; then
                error_exit "Only Apple Silicon macOS binaries are published. Build from source on Intel macOS."
            fi
            ARCH="amd64"
            ;;
        *) error_exit "Unsupported architecture: ${arch}" ;;
    esac

    success "Platform: ${OS}/${ARCH}"
}

fetch_url() {
    url="$1"
    if [ "${DOWNLOADER}" = "curl" ]; then
        curl -sSfL --retry 3 --retry-delay 1 --connect-timeout 10 "${url}" 2>/dev/null
    else
        wget -qO- --tries=3 --timeout=30 "${url}" 2>/dev/null
    fi
}

parse_first_tag() {
    sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1
}

get_latest_version() {
    response="$(fetch_url "${API_BASE_URL}/releases/latest" || true)"
    tag="$(printf '%s\n' "${response}" | parse_first_tag || true)"

    if [ -z "${tag}" ]; then
        response="$(fetch_url "${API_BASE_URL}/releases?per_page=1" || true)"
        tag="$(printf '%s\n' "${response}" | parse_first_tag || true)"
        if [ -n "${tag}" ]; then
            warn "No stable release found; installing pre-release ${tag}"
        fi
    fi

    [ -n "${tag}" ] || error_exit "Could not find a GitHub release for ${REPO}"
    VERSION="${tag}"
    normalize_version
}

build_download_url() {
    version_no_v="${VERSION#v}"
    TARBALL="${BINARY}_${version_no_v}_${OS}_${ARCH}.tar.gz"
    TARBALL_URL="${RELEASE_BASE_URL}/${VERSION}/${TARBALL}"
    CHECKSUMS_URL="${RELEASE_BASE_URL}/${VERSION}/checksums.txt"
}

download_file() {
    url="$1"
    dest="$2"
    if [ "${DOWNLOADER}" = "curl" ]; then
        curl -sSfL --retry 3 --retry-delay 1 --connect-timeout 10 -o "${dest}" "${url}" 2>/dev/null
    else
        wget -q --tries=3 --timeout=30 -O "${dest}" "${url}" 2>/dev/null
    fi
}

verify_checksum() {
    checksums_file="$1"
    tarball_file="$2"

    expected="$(awk -v file="${TARBALL}" '$2 == file { print $1; exit }' "${checksums_file}")"
    [ -n "${expected}" ] || error_exit "Checksum not found for ${TARBALL}"

    if [ "${HASHER}" = "shasum" ]; then
        actual="$(shasum -a 256 "${tarball_file}" | awk '{print $1}')"
    else
        actual="$(sha256sum "${tarball_file}" | awk '{print $1}')"
    fi

    [ "${expected}" = "${actual}" ] || error_exit "Checksum mismatch for ${TARBALL}"
}

install_binary() {
    tmpdir="$1"
    target="${INSTALL_DIR}/${BINARY}"
    target_tmp="${INSTALL_DIR}/.${BINARY}.tmp.$$"

    tar -xzf "${tmpdir}/${TARBALL}" -C "${tmpdir}"
    [ -f "${tmpdir}/${BINARY}" ] || error_exit "Archive did not contain ${BINARY}"

    mkdir -p "${INSTALL_DIR}"

    if command -v install >/dev/null 2>&1; then
        install -m 755 "${tmpdir}/${BINARY}" "${target_tmp}"
    else
        cp "${tmpdir}/${BINARY}" "${target_tmp}"
        chmod 755 "${target_tmp}"
    fi

    mv "${target_tmp}" "${target}"
}

verify_installed_binary() {
    output="$("${INSTALL_DIR}/${BINARY}" --version 2>/dev/null || true)"
    case "${output}" in
        *"${BINARY}"*) success "Verified ${BINARY} runs: ${output}" ;;
        *) error_exit "Installed binary did not run correctly: ${INSTALL_DIR}/${BINARY}" ;;
    esac
}

check_path() {
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) return ;;
    esac

    warn "${INSTALL_DIR} is not in your PATH"

    shell_name="$(basename "${SHELL:-unknown}")"
    case "${shell_name}" in
        zsh)
            rc_file="\$HOME/.zshrc"
            shell_label="zsh"
            ;;
        bash)
            rc_file="\$HOME/.bashrc"
            shell_label="bash"
            ;;
        fish)
            rc_file="\$HOME/.config/fish/config.fish"
            shell_label="fish"
            ;;
        unknown)
            rc_file="your shell startup file"
            shell_label="unknown"
            shell_note="Could not detect your interactive shell from SHELL."
            ;;
        *)
            rc_file="your shell startup file"
            shell_label="${shell_name}"
            shell_note="No built-in startup-file recommendation for ${shell_name}."
            ;;
    esac

    info "Detected shell: ${shell_label}"
    if [ -n "${shell_note:-}" ]; then
        info "${shell_note}"
    fi
    info "Add this to ${rc_file}:"
    printf "\n  %sexport PATH=\"%s:\$PATH\"%s\n\n" "${SUBTEXT}" "${INSTALL_DIR}" "${RESET}"

    info "Common startup files:"
    printf '  %sbash: %s  zsh: %s  fish: %s%s\n\n' \
        "${SUBTEXT}" \
        "\$HOME/.bashrc" \
        "\$HOME/.zshrc" \
        "\$HOME/.config/fish/config.fish" \
        "${RESET}"
}

cleanup() {
    if [ -n "${TMPDIR_CREATED:-}" ]; then
        rm -rf "${TMPDIR_CREATED}"
    fi
}

main() {
    setup_colors
    banner
    parse_args "$@"

    step 1 6 "Checking dependencies"
    check_dependencies

    step 2 6 "Detecting platform"
    detect_platform

    step 3 6 "Resolving release"
    if [ -n "${VERSION}" ]; then
        success "Version: ${VERSION}"
    else
        get_latest_version
        success "Version: ${VERSION} (latest)"
    fi

    build_download_url
    release_panel

    tmpdir="$(mktemp -d)"
    TMPDIR_CREATED="${tmpdir}"
    trap cleanup EXIT HUP INT TERM

    step 4 6 "Downloading ${TARBALL}"
    download_file "${TARBALL_URL}" "${tmpdir}/${TARBALL}" \
        || error_exit "Download failed: ${TARBALL_URL}"
    success "Downloaded ${TARBALL}"

    step 5 6 "Verifying checksum"
    download_file "${CHECKSUMS_URL}" "${tmpdir}/checksums.txt" \
        || error_exit "Failed to download checksums.txt"
    verify_checksum "${tmpdir}/checksums.txt" "${tmpdir}/${TARBALL}"
    success "Checksum verified (SHA-256)"

    step 6 6 "Installing to ${INSTALL_DIR}"
    install_binary "${tmpdir}" \
        || error_exit "Install failed. Try --dir \"${HOME}/.local/bin\" or a writable directory."
    success "Installed ${BINARY} ${VERSION}"
    verify_installed_binary

    if [ "${OS}" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
        xattr -d com.apple.quarantine "${INSTALL_DIR}/${BINARY}" 2>/dev/null || true
        success "Cleared macOS quarantine flag"
    fi

    printf '\n  %s✓%s %s%sInstallation complete.%s\n\n' "${GREEN}" "${RESET}" "${BOLD}" "${TEXT}" "${RESET}"
    check_path
    info "Run ${BINARY} in any git repository to get started"
    info "Press ? for the keybind reference"
    info "Configuration: ~/.config/nidhi/config.toml"
    printf '\n'
}

main "$@"
