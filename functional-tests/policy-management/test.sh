#!/bin/bash
# vim: dict+=/usr/share/beakerlib/dictionary.vim cpt=.,w,b,u,t,i,k
. /usr/share/beakerlib/beakerlib.sh || exit 1

. "$(cd "$(dirname "$0")" && pwd)/../mcp-helpers/lib.sh"

POLICY_NAME="test-mcp-policy"
MB_POLICY_NAME="test-mcp-mb-policy"
TEST_SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

rlJournalStart

    rlPhaseStartSetup "Build and start MCP server"
        rlRun 'rlImport "./test-helpers"' || rlDie "cannot import keylime-tests/test-helpers library"
        rlAssertRpm keylime

        mcpBuildServer
        mcpStartServer
        mcpInitialize

        TESTDIR=$(limeCreateTestDir)
        rlRun "cp ${TEST_SCRIPT_DIR}/testdata/test_runtime_policy.json ${TESTDIR}/test_runtime_policy.json"
        rlRun "cp ${TEST_SCRIPT_DIR}/testdata/test_mb_policy.json ${TESTDIR}/test_mb_policy.json"
    rlPhaseEnd

    rlPhaseStartTest "List_runtime_policies initial state"
        mcpCallTool "List_runtime_policies" '{}' 10
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Import_runtime_policy"
        mcpCallTool "Import_runtime_policy" "{\"name\":\"${POLICY_NAME}\",\"file_path\":\"${TESTDIR}/test_runtime_policy.json\"}" 11
        mcpAssertSuccess
        mcpAssertResultContains 'imported'
    rlPhaseEnd

    rlPhaseStartTest "List_runtime_policies after import"
        mcpCallTool "List_runtime_policies" '{}' 12
        mcpAssertSuccess
        mcpAssertResultContains "${POLICY_NAME}"
    rlPhaseEnd

    rlPhaseStartTest "Get_runtime_policy"
        mcpCallTool "Get_runtime_policy" "{\"policy_name\":\"${POLICY_NAME}\"}" 13
        mcpAssertSuccess
        mcpAssertResultContains "${POLICY_NAME}"
    rlPhaseEnd

    rlPhaseStartTest "Update_runtime_policy add exclude"
        mcpCallTool "Update_runtime_policy" "{\"policy_name\":\"${POLICY_NAME}\",\"add_excludes\":[\"/var/log/test\"]}" 14
        mcpAssertSuccess
        mcpAssertResultContains 'updated'
    rlPhaseEnd

    rlPhaseStartTest "Get_runtime_policy after update"
        mcpCallTool "Get_runtime_policy" "{\"policy_name\":\"${POLICY_NAME}\"}" 15
        mcpAssertSuccess
        mcpAssertResultContains 'var/log/test'
    rlPhaseEnd

    rlPhaseStartTest "Delete_runtime_policy"
        mcpCallTool "Delete_runtime_policy" "{\"policy_name\":\"${POLICY_NAME}\"}" 16
        mcpAssertSuccess
        mcpAssertResultContains 'deleted'
    rlPhaseEnd

    rlPhaseStartTest "List_runtime_policies after delete"
        mcpCallTool "List_runtime_policies" '{}' 17
        mcpAssertResultNotContains "${POLICY_NAME}"
    rlPhaseEnd

    rlPhaseStartTest "List_mb_policies initial state"
        mcpCallTool "List_mb_policies" '{}' 20
        mcpAssertSuccess
    rlPhaseEnd

    rlPhaseStartTest "Import_mb_policy"
        mcpCallTool "Import_mb_policy" "{\"name\":\"${MB_POLICY_NAME}\",\"file_path\":\"${TESTDIR}/test_mb_policy.json\"}" 21
        mcpAssertSuccess
        mcpAssertResultContains 'imported'
    rlPhaseEnd

    rlPhaseStartTest "Get_mb_policy"
        mcpCallTool "Get_mb_policy" "{\"policy_name\":\"${MB_POLICY_NAME}\"}" 22
        mcpAssertSuccess
        mcpAssertResultContains "${MB_POLICY_NAME}"
    rlPhaseEnd

    rlPhaseStartTest "Delete_mb_policy"
        mcpCallTool "Delete_mb_policy" "{\"policy_name\":\"${MB_POLICY_NAME}\"}" 23
        mcpAssertSuccess
        mcpAssertResultContains 'deleted'
    rlPhaseEnd

    rlPhaseStartCleanup "Stop MCP server"
        mcpStopServer
        rlAssertNotGrep "Traceback" "$(limeVerifierLogfile)" || true
        rlAssertNotGrep "Traceback" "$(limeRegistrarLogfile)" || true
        limeSubmitCommonLogs
    rlPhaseEnd

rlJournalEnd
