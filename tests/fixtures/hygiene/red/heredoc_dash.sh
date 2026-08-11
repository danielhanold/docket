#!/usr/bin/env bash
# RED FIXTURE: <<- is the same hazard; only the leading-tab stripping differs.
	cat <<-EOF
	the anchor is `printf HAZARD` here
	EOF
