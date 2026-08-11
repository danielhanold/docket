#!/usr/bin/env bash
# GREEN FIXTURE: a QUOTED heredoc delimiter makes the body inert. All three spellings.
cat <<'EOF'
prose containing `printf INERT` and $notexpanded
EOF
cat <<"EOF"
more prose containing `printf INERT`
EOF
cat <<\EOF
still inert: `printf INERT`
EOF
