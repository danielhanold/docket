#!/usr/bin/env bash
# RED FIXTURE: an UNQUOTED heredoc delimiter substitutes in the body.
cat <<EOF
the anchor is `printf HAZARD` here
EOF
