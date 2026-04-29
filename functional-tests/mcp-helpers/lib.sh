#!/bin/bash

MCP_SRC_DIR="${TMT_TREE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"

mcpBuildServer() {
    rlRun "make -C ${MCP_SRC_DIR} build-server"
    MCP_BIN="${MCP_SRC_DIR}/bin/server"
}

mcpStartServer() {
    MCP_FIFO=$(mktemp -u)
    MCP_OUTPUT=$(mktemp)
    MCP_ERR=$(mktemp)
    rlRun "mkfifo ${MCP_FIFO}"

    local env_prefix=""
    for pair in "$@"; do
        env_prefix="${env_prefix} ${pair}"
    done

    eval "${env_prefix} ${MCP_BIN} < ${MCP_FIFO} > ${MCP_OUTPUT} 2>${MCP_ERR} &"
    MCP_PID=$!
    exec 3>${MCP_FIFO}
    sleep 1
}

mcpStartServerExpectFail() {
    local fifo
    fifo=$(mktemp -u)
    MCP_OUTPUT=$(mktemp)
    MCP_ERR=$(mktemp)
    mkfifo "${fifo}"

    local env_prefix=""
    for pair in "$@"; do
        env_prefix="${env_prefix} ${pair}"
    done

    eval "${env_prefix} ${MCP_BIN} < ${fifo} > ${MCP_OUTPUT} 2>${MCP_ERR} &"
    local pid=$!
    sleep 2

    if ! kill -0 "${pid}" 2>/dev/null; then
        wait "${pid}" 2>/dev/null
        rm -f "${fifo}"
        return 0
    else
        kill "${pid}" 2>/dev/null
        wait "${pid}" 2>/dev/null
        rm -f "${fifo}"
        return 1
    fi
}

mcpStopServer() {
    exec 3>&- 2>/dev/null
    if [ -n "${MCP_PID}" ]; then
        kill "${MCP_PID}" 2>/dev/null
        wait "${MCP_PID}" 2>/dev/null
    fi
    if [ -n "${MCP_OUTPUT}" ]; then
        rlLog "=== MCP server stdout ==="
        head -100 "${MCP_OUTPUT}"
        limeLogfileSubmit "${MCP_OUTPUT}"
    fi
    if [ -n "${MCP_ERR}" ]; then
        rlLog "=== MCP server stderr ==="
        cat "${MCP_ERR}"
        limeLogfileSubmit "${MCP_ERR}"
    fi
    rm -f "${MCP_FIFO}" "${MCP_OUTPUT}" "${MCP_ERR}"
    MCP_PID=""
}

mcpInitialize() {
    echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}},"id":1}' >&3
    sleep 2
    echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&3
    sleep 1
    rlAssertGrep '"serverInfo"' "${MCP_OUTPUT}"
    rlAssertGrep 'Keylime' "${MCP_OUTPUT}"
}

mcpCallTool() {
    local tool_name="$1"
    local arguments="$2"
    local id="$3"
    local wait="${MCP_WAIT:-3}"

    echo "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"${tool_name}\",\"arguments\":${arguments}},\"id\":${id}}" >&3
    sleep "${wait}"
}

mcpSendRaw() {
    echo "$1" >&3
    sleep "${MCP_WAIT:-2}"
}

mcpAssertSuccess() {
    rlAssertNotGrep '"isError":true' "${MCP_OUTPUT}"
}

mcpAssertError() {
    rlAssertGrep '"isError":true' "${MCP_OUTPUT}"
}

mcpAssertResultContains() {
    rlAssertGrep "$1" "${MCP_OUTPUT}"
}

mcpAssertResultNotContains() {
    rlAssertNotGrep "$1" "${MCP_OUTPUT}"
}
