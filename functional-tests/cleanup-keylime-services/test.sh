#!/bin/bash
# vim: dict+=/usr/share/beakerlib/dictionary.vim cpt=.,w,b,u,t,i,k
. /usr/share/beakerlib/beakerlib.sh || exit 1

rlJournalStart

    rlPhaseStartCleanup "Stop keylime services and collect logs"
        rlRun 'rlImport "./test-helpers"' || rlDie "cannot import keylime-tests/test-helpers library"

        rlRun "limeStopAgent"
        rlRun "limeStopRegistrar"
        rlRun "limeStopVerifier"
        if limeTPMEmulated; then
            rlRun "limeStopIMAEmulator"
            rlRun "limeStopTPMEmulator"
            rlRun "limeCondStopAbrmd"
        fi

        rlAssertNotGrep "Traceback" "$(limeVerifierLogfile)" || true
        rlAssertNotGrep "Traceback" "$(limeRegistrarLogfile)" || true

        limeSubmitCommonLogs
        limeClearData
        limeRestoreConfig
    rlPhaseEnd

rlJournalEnd
