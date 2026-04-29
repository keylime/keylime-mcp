#!/bin/bash
# vim: dict+=/usr/share/beakerlib/dictionary.vim cpt=.,w,b,u,t,i,k
. /usr/share/beakerlib/beakerlib.sh || exit 1

AGENT_ID="d432fbb3-d2f1-4a97-9ef7-75bd81c00000"
. "$(cd "$(dirname "$0")" && pwd)/../mcp-helpers/lib.sh"

rlJournalStart

    rlPhaseStartSetup "Build and start MCP server"
        rlRun 'rlImport "./test-helpers"' || rlDie "cannot import keylime-tests/test-helpers library"
        rlAssertRpm keylime

        mcpBuildServer
        mcpStartServer
    rlPhaseEnd

    rlPhaseStartTest "MCP server initialization"
        mcpInitialize
    rlPhaseEnd

    rlPhaseStartTest "MCP tools/list"
        mcpSendRaw '{"jsonrpc":"2.0","method":"tools/list","id":2}'
        mcpAssertResultContains 'Get_version_and_health'
        mcpAssertResultContains 'Get_all_agents'
        mcpAssertResultContains 'Get_agent_status'
        mcpAssertResultContains 'Enroll_agent_to_verifier'
        mcpAssertResultContains 'List_runtime_policies'
        mcpAssertResultContains 'Get_verifier_logs'
    rlPhaseEnd

    rlPhaseStartTest "Get_version_and_health"
        mcpCallTool "Get_version_and_health" '{}' 3
        mcpAssertResultContains 'verifier'
        mcpAssertResultContains 'registrar'
    rlPhaseEnd

    rlPhaseStartTest "Get_all_agents"
        mcpCallTool "Get_all_agents" '{}' 4
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_details"
        mcpCallTool "Get_agent_details" "{\"agent_uuid\":\"${AGENT_ID}\"}" 5
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Enroll_agent_to_verifier"
        mcpCallTool "Enroll_agent_to_verifier" "{\"agent_uuid\":\"${AGENT_ID}\",\"runtime_policy_name\":\"\",\"mb_policy_name\":\"\"}" 6
        MCP_WAIT=5
        mcpAssertSuccess
        MCP_WAIT=3
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_status after enrollment"
        mcpCallTool "Get_agent_status" "{\"agent_uuid\":\"${AGENT_ID}\"}" 7
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_enrolled_agents"
        mcpCallTool "Get_verifier_enrolled_agents" '{}' 8
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "List_runtime_policies"
        mcpCallTool "List_runtime_policies" '{}' 9
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Unenroll_agent_from_verifier"
        mcpCallTool "Unenroll_agent_from_verifier" "{\"agent_uuid\":\"${AGENT_ID}\"}" 10
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartCleanup "Stop MCP server"
        mcpStopServer
        rlAssertNotGrep "Traceback" "$(limeVerifierLogfile)" || true
        rlAssertNotGrep "Traceback" "$(limeRegistrarLogfile)" || true
        limeSubmitCommonLogs
    rlPhaseEnd

rlJournalEnd
