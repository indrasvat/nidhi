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

installed_version="$("${install_dir}/nidhi" --version)"
[ "${installed_version}" = "nidhi 9.8.7 (commit: fake, built: test)" ] || {
    printf 'unexpected installed version: %s\n' "${installed_version}" >&2
    exit 1
}

if NO_COLOR=1 sh "${INSTALLER}" --version "not-semver" --dir "${install_dir}" >/dev/null 2>&1; then
    printf 'invalid SemVer input should fail\n' >&2
    exit 1
fi

printf 'installer smoke test passed\n'
