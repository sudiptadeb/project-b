# Where Project B's signing keys are, for the scripts that source this. The one rule:
#
#   $KEYKOSH_ROOT/project-b  when KEYKOSH_ROOT is set (keykosh, github.com/sudiptadeb/keykosh)
#   <repo>/.keys             otherwise, for someone with keys of their own (git-ignored)
#
# The file names are the same in either: identity.p12, cert.pem, identity.sha1 and
# project-b-trust.mobileconfig (tools/signing-setup.sh), android.keystore
# (tools/android-signing-setup.sh). Their passwords stay in ~/PW_TOOLS/lab-secrets.env.
# A set KEYKOSH_ROOT never falls back to .keys.
KEYS=${KEYKOSH_ROOT:+$KEYKOSH_ROOT/project-b}
KEYS=${KEYS:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.keys}

# Stops the script when a key isn't there, with where it looked.
need_key() {
    [ -e "$KEYS/$1" ] && return 0
    if [ -n "${KEYKOSH_ROOT:-}" ]; then
        echo "no $KEYS/$1: run keykosh unlock" >&2
    else
        echo "no $KEYS/$1: put Project B's keys in $KEYS (tools/keys.sh)" >&2
    fi
    exit 1
}
