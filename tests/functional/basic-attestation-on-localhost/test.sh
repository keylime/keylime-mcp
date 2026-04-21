#!/bin/bash
# vim: dict+=/usr/share/beakerlib/dictionary.vim cpt=.,w,b,u,t,i,k
. /usr/share/beakerlib/beakerlib.sh || exit 1

AGENT_ID="d432fbb3-d2f1-4a97-9ef7-75bd81c00000"
MCP_SRC_DIR="${TMT_TREE:-$(cd $(dirname $0)/../../.. && pwd)}"

rlJournalStart

    rlPhaseStartSetup "Start keylime services and build MCP server"
        rlRun 'rlImport "./test-helpers"' || rlDie "cannot import keylime-tests/test-helpers library"
        rlAssertRpm keylime

        # update /etc/keylime.conf
        limeBackupConfig
        rlRun "limeUpdateConf revocations enabled_revocation_notifications '[]'"
        rlRun "limeUpdateConf tenant require_ek_cert False"
        rlRun "limeUpdateConf agent enable_revocation_notifications false"

        # if TPM emulator is present
        if limeTPMEmulated; then
            rlRun "limeStartTPMEmulator"
            rlRun "limeWaitForTPMEmulator"
            rlRun "limeCondStartAbrmd"
            rlRun "limeInstallIMAConfig"
            rlRun "limeStartIMAEmulator"
        fi
        sleep 5

        # start keylime services
        rlRun "limeStartVerifier"
        rlRun "limeWaitForVerifier"
        rlRun "limeStartRegistrar"
        rlRun "limeWaitForRegistrar"
        rlRun "limeStartAgent"
        rlRun "limeWaitForAgentRegistration ${AGENT_ID}"

        # build MCP server
        TESTDIR=$(limeCreateTestDir)
        rlRun "pushd ${MCP_SRC_DIR}"
        rlRun "go build -o ${TESTDIR}/mcp-server cmd/server/main.go"
        rlRun "popd"

        # prepare MCP communication channel
        MCP_FIFO=$(mktemp -u)
        MCP_OUTPUT=$(mktemp)
        MCP_ERR=$(mktemp)
        rlRun "mkfifo ${MCP_FIFO}"

        # start MCP server
        KEYLIME_VERIFIER_URL=https://localhost:8881 \
        KEYLIME_REGISTRAR_URL=https://localhost:8891 \
        KEYLIME_TLS_ENABLED=true \
        KEYLIME_TLS_SERVER_NAME=localhost \
        KEYLIME_CERT_DIR=/var/lib/keylime/cv_ca \
        KEYLIME_API_VERSION=v2.1 \
        ${TESTDIR}/mcp-server < ${MCP_FIFO} > ${MCP_OUTPUT} 2>${MCP_ERR} &
        MCP_PID=$!
        exec 3>${MCP_FIFO}
        sleep 1
    rlPhaseEnd

    rlPhaseStartTest "MCP server initialization"
        echo '{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test-client","version":"1.0"}},"id":1}' >&3
        sleep 2
        echo '{"jsonrpc":"2.0","method":"notifications/initialized"}' >&3
        sleep 1

        rlAssertGrep '"result"' ${MCP_OUTPUT}
        rlAssertGrep '"serverInfo"' ${MCP_OUTPUT}
        rlAssertGrep 'Keylime' ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "MCP tools/list"
        echo '{"jsonrpc":"2.0","method":"tools/list","id":2}' >&3
        sleep 2

        rlAssertGrep 'Get_version_and_health' ${MCP_OUTPUT}
        rlAssertGrep 'Get_all_agents' ${MCP_OUTPUT}
        rlAssertGrep 'Get_agent_status' ${MCP_OUTPUT}
        rlAssertGrep 'Enroll_agent_to_verifier' ${MCP_OUTPUT}
        rlAssertGrep 'List_runtime_policies' ${MCP_OUTPUT}
        rlAssertGrep 'Get_verifier_logs' ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "Get_version_and_health"
        echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"Get_version_and_health","arguments":{}},"id":3}' >&3
        sleep 3

        rlAssertGrep 'verifier' ${MCP_OUTPUT}
        rlAssertGrep 'registrar' ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "Get_all_agents"
        echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"Get_all_agents","arguments":{}},"id":4}' >&3
        sleep 3

        rlAssertGrep "${AGENT_ID}" ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_details"
        echo "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"Get_agent_details\",\"arguments\":{\"agent_id\":\"${AGENT_ID}\"}},\"id\":5}" >&3
        sleep 3

        rlAssertGrep "${AGENT_ID}" ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "Enroll_agent_to_verifier"
        echo "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"Enroll_agent_to_verifier\",\"arguments\":{\"agent_id\":\"${AGENT_ID}\"}},\"id\":6}" >&3
        sleep 5

        rlAssertNotGrep '"isError":true' ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_status after enrollment"
        echo "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"Get_agent_status\",\"arguments\":{\"agent_id\":\"${AGENT_ID}\"}},\"id\":7}" >&3
        sleep 3

        rlAssertGrep "${AGENT_ID}" ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_enrolled_agents"
        echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"Get_verifier_enrolled_agents","arguments":{}},"id":8}' >&3
        sleep 3

        rlAssertGrep "${AGENT_ID}" ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "List_runtime_policies"
        echo '{"jsonrpc":"2.0","method":"tools/call","params":{"name":"List_runtime_policies","arguments":{}},"id":9}' >&3
        sleep 3

        rlAssertNotGrep '"isError":true' ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartTest "Unenroll_agent_from_verifier"
        echo "{\"jsonrpc\":\"2.0\",\"method\":\"tools/call\",\"params\":{\"name\":\"Unenroll_agent_from_verifier\",\"arguments\":{\"agent_id\":\"${AGENT_ID}\"}},\"id\":10}" >&3
        sleep 3

        rlAssertNotGrep '"isError":true' ${MCP_OUTPUT}
    rlPhaseEnd

    rlPhaseStartCleanup "Stop MCP server and keylime services"
        # close MCP server stdin
        exec 3>&-
        rlRun "kill ${MCP_PID} 2>/dev/null; wait ${MCP_PID} 2>/dev/null" 0,1

        rlLog "=== MCP server stderr ==="
        rlRun "cat ${MCP_ERR}"
        limeLogfileSubmit "${MCP_OUTPUT}"
        limeLogfileSubmit "${MCP_ERR}"
        rlRun "rm -f ${MCP_FIFO} ${MCP_OUTPUT} ${MCP_ERR}"

        rlRun "limeStopAgent"
        rlRun "limeStopRegistrar"
        rlRun "limeStopVerifier"
        if limeTPMEmulated; then
            rlRun "limeStopIMAEmulator"
            rlRun "limeStopTPMEmulator"
            rlRun "limeCondStopAbrmd"
        fi
        limeSubmitCommonLogs
        limeClearData
        limeRestoreConfig
    rlPhaseEnd

rlJournalEnd
