#!/bin/bash
# vim: dict+=/usr/share/beakerlib/dictionary.vim cpt=.,w,b,u,t,i,k
. /usr/share/beakerlib/beakerlib.sh || exit 1

FAKE_UUID="00000000-0000-0000-0000-000000000000"
. "$(cd "$(dirname "$0")" && pwd)/../mcp-helpers/lib.sh"

rlJournalStart

    rlPhaseStartSetup "Build and start MCP server"
        rlRun 'rlImport "./test-helpers"' || rlDie "cannot import keylime-tests/test-helpers library"
        rlAssertRpm keylime

        mcpBuildServer
        mcpStartServer
        mcpInitialize
    rlPhaseEnd

    rlPhaseStartTest "Invalid UUID format returns error"
        mcpCallTool "Get_agent_status" '{"agent_uuid":"not-a-uuid"}' 10
        mcpAssertError
    rlPhaseEnd

    rlPhaseStartTest "Non-existent agent returns error"
        mcpCallTool "Get_agent_status" "{\"agent_uuid\":\"${FAKE_UUID}\"}" 11
        mcpAssertError
    rlPhaseEnd

    rlPhaseStartTest "Enroll non-existent agent returns error"
        mcpCallTool "Enroll_agent_to_verifier" "{\"agent_uuid\":\"${FAKE_UUID}\"}" 12
        mcpAssertError
    rlPhaseEnd

    rlPhaseStartTest "Unenroll non-enrolled agent returns error"
        mcpCallTool "Unenroll_agent_from_verifier" "{\"agent_uuid\":\"${FAKE_UUID}\"}" 13
        mcpAssertError
    rlPhaseEnd

    rlPhaseStartTest "Get non-existent runtime policy returns error"
        mcpCallTool "Get_runtime_policy" '{"policy_name":"nonexistent-policy"}' 14
        mcpAssertError
    rlPhaseEnd

    rlPhaseStartTest "Delete non-existent runtime policy returns error"
        mcpCallTool "Delete_runtime_policy" '{"policy_name":"nonexistent-policy"}' 15
        mcpAssertError
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_logs with attestation_failures filter"
        mcpCallTool "Get_verifier_logs" '{"filter":"attestation_failures","lines":10}' 16
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_logs with errors filter"
        mcpCallTool "Get_verifier_logs" '{"filter":"errors","lines":10}' 17
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_logs with all filter"
        mcpCallTool "Get_verifier_logs" '{"filter":"all","lines":10}' 18
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Partial service failure - verifier down"
        rlRun "limeStopVerifier"
        sleep 2
        mcpCallTool "Get_version_and_health" '{}' 19
        mcpAssertSuccess
        mcpAssertResultContains '"reachable":false'
        mcpAssertResultContains '"reachable":true'
        rlRun "limeStartVerifier"
        rlRun "limeWaitForVerifier"
    rlPhaseEnd

    rlPhaseStartCleanup "Ensure services running and stop MCP server"
        if ! limeWaitForVerifier 2>/dev/null; then
            rlRun "limeStartVerifier"
            rlRun "limeWaitForVerifier"
        fi
        mcpStopServer
        rlAssertNotGrep "Traceback" "$(limeVerifierLogfile)" || true
        rlAssertNotGrep "Traceback" "$(limeRegistrarLogfile)" || true
        limeSubmitCommonLogs
    rlPhaseEnd

rlJournalEnd
