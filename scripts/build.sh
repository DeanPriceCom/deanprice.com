#!/usr/bin/env bash
set -euo pipefail

echo "==> Starting build pipeline..."

# 0. Resolve Go command
GO_CMD="go"
if ! command -v go >/dev/null 2>&1 && command -v go.exe >/dev/null 2>&1; then
    GO_CMD="go.exe"
fi

TOOLS_DIR="$(pwd)/.tools"
mkdir -p "${TOOLS_DIR}"

# 1. Resolve & Cache TinyGo if not in PATH
TINYGO_CMD="tinygo"
if ! command -v tinygo >/dev/null 2>&1; then
    if command -v tinygo.exe >/dev/null 2>&1; then
        TINYGO_CMD="tinygo.exe"
    else
        TINYGO_VERSION="0.41.1"
        if [ ! -d "${TOOLS_DIR}/tinygo/bin" ]; then
            echo "==> TinyGo not found in PATH. Downloading TinyGo v${TINYGO_VERSION}..."
            curl -fsSL "https://github.com/tinygo-org/tinygo/releases/download/v${TINYGO_VERSION}/tinygo${TINYGO_VERSION}.linux-amd64.tar.gz" -o tinygo.tar.gz
            tar -xzf tinygo.tar.gz -C "${TOOLS_DIR}"
            rm -f tinygo.tar.gz
        fi
        export PATH="${TOOLS_DIR}/tinygo/bin:${PATH}"
    fi
fi

echo "==> Using Go: $(${GO_CMD} version)"
echo "==> Using TinyGo: $(${TINYGO_CMD} version)"

# 2. Resolve & Cache Binaryen (wasm-opt) if not in PATH
WASM_OPT_CMD="wasm-opt"
if ! command -v wasm-opt >/dev/null 2>&1; then
    if command -v wasm-opt.exe >/dev/null 2>&1; then
        WASM_OPT_CMD="wasm-opt.exe"
    else
        BINARYEN_VERSION="version_126"
        BINARYEN_DIR="binaryen-${BINARYEN_VERSION}"
        if [ ! -d "${TOOLS_DIR}/${BINARYEN_DIR}/bin" ]; then
            echo "==> wasm-opt not found in PATH. Downloading Binaryen ${BINARYEN_VERSION}..."
            curl -fsSL "https://github.com/WebAssembly/binaryen/releases/download/${BINARYEN_VERSION}/binaryen-${BINARYEN_VERSION}-x86_64-linux.tar.gz" -o binaryen.tar.gz
            tar -xzf binaryen.tar.gz -C "${TOOLS_DIR}"
            rm -f binaryen.tar.gz
        fi
        export PATH="${TOOLS_DIR}/${BINARYEN_DIR}/bin:${PATH}"
    fi
fi

echo "==> Using wasm-opt: $(${WASM_OPT_CMD} --version)"

# 3. Generate obfuscated keystreams and sync CSP in _headers
echo "==> Running go generate..."
"${GO_CMD}" generate .

# 4. Compile Micro-WASM binary with TinyGo
echo "==> Compiling WebAssembly binary with TinyGo..."
"${TINYGO_CMD}" build -opt=z -no-debug -panic=trap -target wasm -o main.wasm .

# 5. Post-optimize with wasm-opt
echo "==> Optimizing with wasm-opt -Oz..."
"${WASM_OPT_CMD}" -Oz --strip-debug --strip-dwarf --strip-producers -o main.wasm main.wasm

# 6. Ensure wasm_exec.js exists in workspace root
if [ ! -f wasm_exec.js ]; then
    TINYGOROOT="$(${TINYGO_CMD} env TINYGOROOT)"
    echo "==> Copying wasm_exec.js from ${TINYGOROOT}..."
    cp "${TINYGOROOT}/targets/wasm_exec.js" .
fi

# 7. Assemble isolated dist/ deployment bundle
echo "==> Assembling isolated dist/ bundle..."
"${GO_CMD}" run ./tools -task=dist

echo "==> Build completed successfully!"
