#!/bin/bash
# vim: dict+=/usr/share/beakerlib/dictionary.vim cpt=.,w,b,u,t,i,k
. /usr/share/beakerlib/beakerlib.sh || exit 1

AGENT_ID="d432fbb3-d2f1-4a97-9ef7-75bd81c00000"

rlJournalStart

    rlPhaseStartSetup "Start keylime services"
        rlRun 'rlImport "./test-helpers"' || rlDie "cannot import keylime-tests/test-helpers library"
        rlAssertRpm keylime

        limeBackupConfig
        rlRun "limeUpdateConf revocations enabled_revocation_notifications '[]'"
        rlRun "limeUpdateConf tenant require_ek_cert False"
        rlRun "limeUpdateConf agent enable_revocation_notifications false"

        if limeTPMEmulated; then
            rlRun "limeStartTPMEmulator"
            rlRun "limeWaitForTPMEmulator"
            rlRun "limeCondStartAbrmd"
            rlRun "limeInstallIMAConfig"
            rlRun "limeStartIMAEmulator"
        fi
        sleep 5

        rlRun "limeStartVerifier"
        rlRun "limeWaitForVerifier"
        rlRun "limeStartRegistrar"
        rlRun "limeWaitForRegistrar"
        rlRun "limeStartAgent"
        rlRun "limeWaitForAgentRegistration ${AGENT_ID}"

        # Generate test policies using upstream keylime-tests helpers
        rlRun "limeCreateTestPolicy /etc/hostname"
        rlRun "mkdir -p /var/tmp/keylime-mcp-tests"
        rlRun "cp policy.json /var/tmp/keylime-mcp-tests/runtime_policy.json"
        rlRun "echo '{}' > /var/tmp/keylime-mcp-tests/mb_policy.json"
    rlPhaseEnd

rlJournalEnd
