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
        mcpInitialize
    rlPhaseEnd

    rlPhaseStartTest "Get_all_agents shows registered agent"
        mcpCallTool "Get_all_agents" '{}' 10
        mcpAssertSuccess
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_details returns registrar info"
        mcpCallTool "Get_agent_details" "{\"agent_uuid\":\"${AGENT_ID}\"}" 11
        mcpAssertSuccess
        mcpAssertResultContains 'aik_tpm'
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_enrolled_agents before enrollment"
        mcpCallTool "Get_verifier_enrolled_agents" '{}' 12
        mcpAssertSuccess
        mcpAssertResultNotContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Enroll_agent_to_verifier"
        mcpCallTool "Enroll_agent_to_verifier" "{\"agent_uuid\":\"${AGENT_ID}\",\"runtime_policy_name\":\"\",\"mb_policy_name\":\"\"}" 13
        MCP_WAIT=5
        mcpAssertSuccess
        MCP_WAIT=3
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_status after enrollment"
        sleep 3
        mcpCallTool "Get_agent_status" "{\"agent_uuid\":\"${AGENT_ID}\"}" 14
        mcpAssertSuccess
        mcpAssertResultContains "${AGENT_ID}"
        mcpAssertResultContains 'operational_state'
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_enrolled_agents after enrollment"
        mcpCallTool "Get_verifier_enrolled_agents" '{}' 15
        mcpAssertSuccess
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_policies"
        mcpCallTool "Get_agent_policies" "{\"agent_uuid\":\"${AGENT_ID}\"}" 16
        mcpAssertSuccess
        mcpAssertResultContains 'tpm_policy'
    rlPhaseEnd

    rlPhaseStartTest "Get_failed_agents returns empty for healthy agent"
        mcpCallTool "Get_failed_agents" '{}' 17
        mcpAssertSuccess
        mcpAssertResultNotContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Stop_agent"
        mcpCallTool "Stop_agent" "{\"agent_uuid\":\"${AGENT_ID}\"}" 18
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Reactivate_agent"
        mcpCallTool "Reactivate_agent" "{\"agent_uuid\":\"${AGENT_ID}\"}" 19
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Update_agent re-enrolls with same config"
        mcpCallTool "Update_agent" "{\"agent_uuid\":\"${AGENT_ID}\",\"runtime_policy_name\":\"\",\"mb_policy_name\":\"\"}" 20
        MCP_WAIT=5
        mcpAssertSuccess
        mcpAssertResultContains 'updated'
        MCP_WAIT=3
    rlPhaseEnd

    rlPhaseStartTest "Get_agent_status after update"
        sleep 3
        mcpCallTool "Get_agent_status" "{\"agent_uuid\":\"${AGENT_ID}\"}" 21
        mcpAssertSuccess
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Unenroll_agent_from_verifier"
        mcpCallTool "Unenroll_agent_from_verifier" "{\"agent_uuid\":\"${AGENT_ID}\"}" 22
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Get_verifier_enrolled_agents after unenroll"
        mcpCallTool "Get_verifier_enrolled_agents" '{}' 23
        mcpAssertSuccess
        mcpAssertResultNotContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Get_all_agents still shows agent in registrar"
        mcpCallTool "Get_all_agents" '{}' 24
        mcpAssertSuccess
        mcpAssertResultContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartTest "Registrar_remove_agent"
        mcpCallTool "Registrar_remove_agent" "{\"agent_uuid\":\"${AGENT_ID}\"}" 25
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Get_all_agents after registrar removal"
        mcpCallTool "Get_all_agents" '{}' 26
        mcpAssertSuccess
        mcpAssertResultNotContains "${AGENT_ID}"
    rlPhaseEnd

    rlPhaseStartCleanup "Stop MCP server"
        mcpStopServer
        rlAssertNotGrep "Traceback" "$(limeVerifierLogfile)" || true
        rlAssertNotGrep "Traceback" "$(limeRegistrarLogfile)" || true
        limeSubmitCommonLogs
    rlPhaseEnd

rlJournalEnd
