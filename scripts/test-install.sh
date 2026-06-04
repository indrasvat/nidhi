#!/bin/sh
set -eu

ROOT_DIR="$(unset CDPATH; cd -- "$(dirname -- "$0")/.." && pwd)"
INSTALLER="${ROOT_DIR}/install.sh"

tmpdir="$(mktemp -d)"
cleanup() {
    rm -rf "${tmpdir}"
}
trap cleanup EXIT HUP INT TERM

os="$(uname -s | tr '[:upper:]' '[:lower:]')"
arch="$(uname -m)"
case "${arch}" in
    arm64|aarch64) arch="arm64" ;;
    x86_64|amd64) arch="amd64" ;;
    *) printf 'unsupported test architecture: %s\n' "${arch}" >&2; exit 1 ;;
esac

if [ "${os}" = "darwin" ] && [ "${arch}" != "arm64" ]; then
    printf 'installer intentionally supports only darwin/arm64; skipping on %s/%s\n' "${os}" "${arch}"
    exit 0
fi

version="v9.8.7"
archive="nidhi_${version#v}_${os}_${arch}.tar.gz"
install_dir="${tmpdir}/install dir"
env_install_dir="${tmpdir}/env install dir"

mkdir -p "${tmpdir}/api/releases" "${tmpdir}/releases/${version}" "${tmpdir}/pkg" "${install_dir}"

printf '{"tag_name":"%s"}\n' "${version}" > "${tmpdir}/api/releases/latest"
cat > "${tmpdir}/pkg/nidhi" <<'EOF'
#!/bin/sh
printf 'nidhi 9.8.7 (commit: fake, built: test)\n'
EOF
chmod +x "${tmpdir}/pkg/nidhi"

tar -C "${tmpdir}/pkg" -czf "${tmpdir}/releases/${version}/${archive}" nidhi

if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${tmpdir}/releases/${version}/${archive}" | awk -v name="${archive}" '{ print $1 "  " name }' > "${tmpdir}/releases/${version}/checksums.txt"
else
    sha256sum "${tmpdir}/releases/${version}/${archive}" | awk -v name="${archive}" '{ print $1 "  " name }' > "${tmpdir}/releases/${version}/checksums.txt"
fi

output="$(
    NIDHI_INSTALL_API_BASE_URL="file://${tmpdir}/api" \
    NIDHI_INSTALL_RELEASE_BASE_URL="file://${tmpdir}/releases" \
    NO_COLOR=1 \
    SHELL=/bin/bash \
    sh "${INSTALLER}" --version "${version}" --dir "${install_dir}"
)"

printf '%s\n' "${output}" | grep -q "Installation complete" || {
    printf 'installer output did not report completion\n%s\n' "${output}" >&2
    exit 1
}

printf '%s\n' "${output}" | grep -q "Verified nidhi runs" || {
    printf 'installer output did not verify the installed binary\n%s\n' "${output}" >&2
    exit 1
}

printf '%s\n' "${output}" | grep -q "Detected shell: bash" || {
    printf 'installer output did not report detected bash shell\n%s\n' "${output}" >&2
    exit 1
}

# shellcheck disable=SC2016 # literal $HOME is expected installer output.
printf '%s\n' "${output}" | grep -F -q 'Add this to $HOME/.bashrc' || {
    printf 'installer output did not recommend .bashrc for bash\n%s\n' "${output}" >&2
    exit 1
}

# shellcheck disable=SC2016 # literal $HOME is expected installer output.
printf '%s\n' "${output}" | grep -F -q 'bash: $HOME/.bashrc  zsh: $HOME/.zshrc  fish: $HOME/.config/fish/config.fish' || {
    printf 'installer output did not include common startup files\n%s\n' "${output}" >&2
    exit 1
}

installed_version="$("${install_dir}/nidhi" --version)"
[ "${installed_version}" = "nidhi 9.8.7 (commit: fake, built: test)" ] || {
    printf 'unexpected installed version: %s\n' "${installed_version}" >&2
    exit 1
}

env_output="$(
    NIDHI_INSTALL_API_BASE_URL="file://${tmpdir}/api" \
    NIDHI_INSTALL_RELEASE_BASE_URL="file://${tmpdir}/releases" \
    NIDHI_INSTALL_DIR="${env_install_dir}" \
    NO_COLOR=1 \
    SHELL=/bin/zsh \
    sh "${INSTALLER}" --version "${version#v}"
)"

printf '%s\n' "${env_output}" | grep -q "Version: ${version}" || {
    printf 'installer did not normalize version without v prefix\n%s\n' "${env_output}" >&2
    exit 1
}

# shellcheck disable=SC2016 # literal $HOME is expected installer output.
printf '%s\n' "${env_output}" | grep -F -q 'Add this to $HOME/.zshrc' || {
    printf 'installer output did not recommend .zshrc for zsh\n%s\n' "${env_output}" >&2
    exit 1
}

"${env_install_dir}/nidhi" --version >/dev/null || {
    printf 'installer did not honor NIDHI_INSTALL_DIR\n' >&2
    exit 1
}

unknown_output="$(
    NIDHI_INSTALL_API_BASE_URL="file://${tmpdir}/api" \
    NIDHI_INSTALL_RELEASE_BASE_URL="file://${tmpdir}/releases" \
    NO_COLOR=1 \
    SHELL='' \
    sh "${INSTALLER}" --version "${version}" --dir "${tmpdir}/unknown shell dir"
)"

printf '%s\n' "${unknown_output}" | grep -q "Detected shell: unknown" || {
    printf 'installer output did not report unknown shell fallback\n%s\n' "${unknown_output}" >&2
    exit 1
}

printf '%s\n' "${unknown_output}" | grep -q "Could not detect your interactive shell from SHELL" || {
    printf 'installer output did not include explicit unknown shell guidance\n%s\n' "${unknown_output}" >&2
    exit 1
}

if NO_COLOR=1 sh "${INSTALLER}" --version "not-semver" --dir "${install_dir}" >/dev/null 2>&1; then
    printf 'invalid SemVer input should fail\n' >&2
    exit 1
fi

printf 'installer smoke test passed\n'
