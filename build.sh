#!/bin/bash
# Build script for yasinstall - Multi-platform build
# Supports: Linux (amd64, arm64), Windows (amd64, arm64), macOS (amd64, arm64)

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BINARY_NAME="yinstall"
BUILD_DIR="build"
CMD_PATH="./cmd/yinstall"
VERSION_FILE="cmd/yinstall/version.go"

TZ_CN="Asia/Shanghai"
VERSION=$(TZ=$TZ_CN date '+%Y%m%d_%H%M%S')
BUILD_TIME=$(TZ=$TZ_CN date '+%Y-%m-%d %H:%M:%S CST')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS="-s -w \
    -X 'main.Version=${VERSION}' \
    -X 'main.BuildTime=${BUILD_TIME}' \
    -X 'main.GitCommit=${GIT_COMMIT}'"
BUILDFLAGS="-trimpath"

print_msg() {
    local color=$1
    shift
    echo -e "${color}$@${NC}"
}

print_header() {
    echo ""
    print_msg "$BLUE" "=========================================="
    print_msg "$BLUE" "$@"
    print_msg "$BLUE" "=========================================="
    echo ""
}

build_platform() {
    local os=$1
    local arch=$2
    local ext=$3

    local output_name="${BINARY_NAME}_${os}_${arch}${ext}"
    local output_path="${BUILD_DIR}/${output_name}"

    print_msg "$YELLOW" "Building ${os}/${arch}..."

    GOOS=$os GOARCH=$arch go build $BUILDFLAGS -ldflags "$LDFLAGS" -o "$output_path" $CMD_PATH

    if [ $? -eq 0 ]; then
        local size_before=$(du -h "$output_path" | cut -f1)
        print_msg "$GREEN" "✓ Built: ${output_name} (${size_before})"

        if [ "$os" = "darwin" ] && [ "$arch" = "arm64" ]; then
            compress_macos_arm64 "$output_path"
        elif [ "$os" = "windows" ] && [ "$arch" = "arm64" ]; then
            print_msg "$YELLOW" "  ⚠ UPX compression not supported for Windows ARM64"
        else
            compress_binary "$output_path" "$os"
        fi

        # macOS: 若目标目录存在，则复制并改名为 yinstall
        if [ "$os" = "darwin" ] && [ "$arch" = "arm64" ]; then
            local target_dir="/Users/yihan/Documents/owner/wendang/home"
            if [ -d "$target_dir" ]; then
                print_msg "$YELLOW" "Copying to ${target_dir}/yinstall..."
                cp "$output_path" "${target_dir}/yinstall"
                if [ $? -eq 0 ]; then
                    print_msg "$GREEN" "✓ Copied to ${target_dir}/yinstall"
                else
                    print_msg "$RED" "✗ Failed to copy to ${target_dir}"
                fi
            fi
        fi
    else
        print_msg "$RED" "✗ Failed to build ${os}/${arch}"
        return 1
    fi
}

compress_binary() {
    local binary_path=$1
    local os=$2

    if ! command -v upx &> /dev/null; then
        return 0
    fi

    local size_before=$(stat -f%z "$binary_path" 2>/dev/null || stat -c%s "$binary_path" 2>/dev/null)
    print_msg "$YELLOW" "  Compressing with UPX..."

    if [ "$os" = "darwin" ]; then
        codesign --remove-signature "$binary_path" 2>/dev/null
        upx --best --lzma --force-macos "$binary_path" >/dev/null 2>&1
        local upx_result=$?
        if [ $upx_result -eq 0 ]; then
            codesign -s - "$binary_path" >/dev/null 2>&1
            if [ $? -eq 0 ]; then
                local size_after=$(stat -f%z "$binary_path" 2>/dev/null || stat -c%s "$binary_path" 2>/dev/null)
                local reduction=$((($size_before - $size_after) * 100 / $size_before))
                print_msg "$GREEN" "  ✓ Compressed and signed (reduced by ${reduction}%)"
            else
                print_msg "$YELLOW" "  ⚠ Compressed but signing failed"
            fi
        else
            print_msg "$YELLOW" "  ⚠ UPX compression not supported for this architecture"
        fi
    else
        upx --best --lzma "$binary_path" >/dev/null 2>&1
        if [ $? -eq 0 ]; then
            local size_after=$(stat -f%z "$binary_path" 2>/dev/null || stat -c%s "$binary_path" 2>/dev/null)
            local reduction=$((($size_before - $size_after) * 100 / $size_before))
            print_msg "$GREEN" "  ✓ Compressed (reduced by ${reduction}%)"
        else
            print_msg "$YELLOW" "  ⚠ UPX compression failed"
        fi
    fi
}

compress_macos_arm64() {
    local binary_path=$1
    local size_before=$(stat -f%z "$binary_path" 2>/dev/null)

    print_msg "$YELLOW" "  Compressing macOS ARM64 binary..."

    if command -v gzip &> /dev/null; then
        local temp_dir=$(mktemp -d)
        local compressed="${temp_dir}/payload.gz"
        local wrapper="${temp_dir}/wrapper.sh"

        gzip -9 -c "$binary_path" > "$compressed"
        cat > "$wrapper" << 'WRAPPER_EOF'
#!/bin/bash
PAYLOAD_LINE=$(awk '/^__PAYLOAD_BEGIN__/ {print NR + 1; exit 0; }' "$0")
TEMP_BIN=$(mktemp)
tail -n +${PAYLOAD_LINE} "$0" | gunzip > "$TEMP_BIN"
chmod +x "$TEMP_BIN"
"$TEMP_BIN" "$@"
EXIT_CODE=$?
rm -f "$TEMP_BIN"
exit $EXIT_CODE
__PAYLOAD_BEGIN__
WRAPPER_EOF

        cat "$wrapper" "$compressed" > "${binary_path}.new"
        chmod +x "${binary_path}.new"
        codesign -s - "${binary_path}.new" >/dev/null 2>&1

        local size_after=$(stat -f%z "${binary_path}.new" 2>/dev/null)
        if [ $size_after -lt $size_before ]; then
            mv "${binary_path}.new" "$binary_path"
            local reduction=$((($size_before - $size_after) * 100 / $size_before))
            print_msg "$GREEN" "  ✓ Compressed with gzip wrapper (reduced by ${reduction}%)"
        else
            rm -f "${binary_path}.new"
            print_msg "$YELLOW" "  ⚠ Gzip wrapper larger than original, keeping original"
        fi
        rm -rf "$temp_dir"
    else
        print_msg "$YELLOW" "  ⚠ gzip not available, keeping original binary"
    fi
}

clean_build() {
    print_msg "$YELLOW" "Cleaning build directory..."
    rm -rf "$BUILD_DIR"
    mkdir -p "$BUILD_DIR"
    print_msg "$GREEN" "✓ Clean complete"
}

show_help() {
    cat << EOF
Usage: $0 [OPTIONS]

Build yinstall for multiple platforms.

OPTIONS:
    -h, --help          Show this help
    -c, --clean         Clean build directory before building
    -l, --linux         Build for Linux only
    -w, --windows       Build for Windows only
    -m, --macos         Build for macOS only
    -a, --all           Build for all platforms (default)
    --current           Build for current platform only

EXAMPLES:
    $0                  # Build all platforms
    $0 --clean          # Clean and build all
    $0 --linux          # Linux only
    $0 --current        # Current platform only

OUTPUT: ${BUILD_DIR}/
        ${BINARY_NAME}_<os>_<arch>[.exe]

EOF
}

build_all() {
    print_header "Building for all platforms"
    build_platform "linux" "amd64" ""
    build_platform "linux" "arm64" ""
    build_platform "windows" "amd64" ".exe"
    build_platform "windows" "arm64" ".exe"
    build_platform "darwin" "amd64" ""
    build_platform "darwin" "arm64" ""
}

build_linux() {
    print_header "Building for Linux"
    build_platform "linux" "amd64" ""
    build_platform "linux" "arm64" ""
}

build_windows() {
    print_header "Building for Windows"
    build_platform "windows" "amd64" ".exe"
    build_platform "windows" "arm64" ".exe"
}

build_macos() {
    print_header "Building for macOS"
    build_platform "darwin" "amd64" ""
    build_platform "darwin" "arm64" ""
}

build_current() {
    print_header "Building for current platform"
    local current_os=$(go env GOOS)
    local current_arch=$(go env GOARCH)
    local ext=""
    [ "$current_os" = "windows" ] && ext=".exe"
    build_platform "$current_os" "$current_arch" "$ext"
}

show_summary() {
    echo ""
    print_header "Build Summary"
    if [ -d "$BUILD_DIR" ]; then
        print_msg "$GREEN" "Build directory: ${BUILD_DIR}/"
        echo ""
        ls -lh "$BUILD_DIR"/ | tail -n +2 | while read -r line; do echo "  $line"; done
        echo ""
        local total_size=$(du -sh "$BUILD_DIR" | cut -f1)
        print_msg "$BLUE" "Total size: ${total_size}"
    else
        print_msg "$RED" "No build directory found"
    fi
    echo ""
}

update_version_file() {
    print_msg "$YELLOW" "Updating version information..."
    cat > "$VERSION_FILE" << EOF
package main

// Version information - auto-generated by build.sh
var (
	Version   = "${VERSION}"
	BuildTime = "${BUILD_TIME}"
	GitCommit = "${GIT_COMMIT}"
)
EOF
    if [ $? -eq 0 ]; then
        print_msg "$GREEN" "✓ Version updated: ${VERSION}"
    else
        print_msg "$RED" "✗ Failed to update version file"
        return 1
    fi
}

main() {
    if ! command -v go &> /dev/null; then
        print_msg "$RED" "Error: Go is not installed"
        exit 1
    fi

    local do_clean=false
    local build_target="all"

    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                show_help
                exit 0
                ;;
            -c|--clean)
                do_clean=true
                shift
                ;;
            -l|--linux)
                build_target="linux"
                shift
                ;;
            -w|--windows)
                build_target="windows"
                shift
                ;;
            -m|--macos)
                build_target="macos"
                shift
                ;;
            -a|--all)
                build_target="all"
                shift
                ;;
            --current)
                build_target="current"
                shift
                ;;
            *)
                print_msg "$RED" "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done

    print_header "yinstall Build Script"
    print_msg "$BLUE" "Version:    ${VERSION}"
    print_msg "$BLUE" "Build Time: ${BUILD_TIME}"
    print_msg "$BLUE" "Git Commit: ${GIT_COMMIT}"
    echo ""

    update_version_file
    echo ""

    if [ "$do_clean" = true ]; then
        clean_build
    else
        mkdir -p "$BUILD_DIR"
    fi

    case $build_target in
        all)      build_all ;;
        linux)    build_linux ;;
        windows)  build_windows ;;
        macos)    build_macos ;;
        current)  build_current ;;
    esac

    show_summary
    print_msg "$GREEN" "✓ Build process complete!"
}

main "$@"
