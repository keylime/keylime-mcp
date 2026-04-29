#!/bin/bash
# vim: dict+=/usr/share/beakerlib/dictionary.vim cpt=.,w,b,u,t,i,k
. /usr/share/beakerlib/beakerlib.sh || exit 1

. "$(cd "$(dirname "$0")" && pwd)/../mcp-helpers/lib.sh"

rlJournalStart

    rlPhaseStartSetup "Build MCP server and generate test certificates"
        rlRun 'rlImport "./test-helpers"' || rlDie "cannot import keylime-tests/test-helpers library"
        rlAssertRpm keylime

        mcpBuildServer

        CERTDIR=$(mktemp -d)
        rlRun "openssl genrsa -out ${CERTDIR}/wrong-ca-key.pem 2048"
        rlRun "openssl req -new -x509 -key ${CERTDIR}/wrong-ca-key.pem -out ${CERTDIR}/wrong-ca-cert.pem -days 1 -subj '/CN=Wrong-Test-CA'"
    rlPhaseEnd

    rlPhaseStartTest "Valid mTLS connection succeeds"
        mcpStartServer
        mcpInitialize
        mcpCallTool "Get_version_and_health" '{}' 2
        mcpAssertSuccess
        mcpAssertResultContains 'verifier'
        mcpAssertResultContains 'registrar'
        mcpStopServer
    rlPhaseEnd

    rlPhaseStartTest "Wrong CA certificate causes TLS error"
        mcpStartServer "KEYLIME_CA_CERT=${CERTDIR}/wrong-ca-cert.pem"
        mcpInitialize
        mcpCallTool "Get_version_and_health" '{}' 2
        mcpAssertResultContains '"reachable":false'
        mcpStopServer
    rlPhaseEnd

    rlPhaseStartTest "Missing client certificate prevents server startup"
        rlRun "mcpStartServerExpectFail KEYLIME_CLIENT_CERT=/nonexistent/client-cert.pem" 0 \
            "Server should fail to start with missing client cert"
        rlAssertGrep "certificate" "${MCP_ERR}" -i
    rlPhaseEnd

    rlPhaseStartTest "SNI hostname mismatch causes TLS error"
        mcpStartServer "KEYLIME_TLS_SERVER_NAME=wrong-hostname.example.com"
        mcpInitialize
        mcpCallTool "Get_version_and_health" '{}' 2
        mcpAssertResultContains '"reachable":false'
        mcpStopServer
    rlPhaseEnd

    rlPhaseStartTest "TLS disabled mode starts without certificates"
        mcpStartServer "KEYLIME_TLS_ENABLED=false"
        mcpInitialize
        mcpCallTool "Get_version_and_health" '{}' 2
        rlAssertNotGrep "failed to load" "${MCP_ERR}"
        rlAssertNotGrep "TLS configuration failed" "${MCP_ERR}"
        mcpStopServer
    rlPhaseEnd

    rlPhaseStartCleanup "Clean up"
        mcpStopServer
        rlRun "rm -rf ${CERTDIR}"
        rlAssertNotGrep "Traceback" "$(limeVerifierLogfile)" || true
        rlAssertNotGrep "Traceback" "$(limeRegistrarLogfile)" || true
        limeSubmitCommonLogs
    rlPhaseEnd

rlJournalEnd
