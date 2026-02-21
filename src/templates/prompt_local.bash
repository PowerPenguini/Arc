### ARC_PROMPT_START
# ARC PROMPT (powerline style, theme aligned)
# Requires a font with powerline/nerd glyphs.
[[ $- != *i* ]] && return

__arc_sw_connect() {
	local __arc_tmux_session="${1:-arc}"
	if [[ ! "$__arc_tmux_session" =~ ^[A-Za-z0-9._-]+$ ]]; then
		printf 'sw: invalid session name %q (allowed: letters, digits, ., _, -)\n' "$__arc_tmux_session" >&2
		return 2
	fi

	# Quiet preflight checks to avoid noisy SSH errors on fallback.
	local __arc_ssh_probe_opts=(-o BatchMode=yes -o ConnectTimeout=2 -o ConnectionAttempts=1 -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR)
	# Fast dead-link detection for the real session so disconnects don't "hang" for long.
	local __arc_ssh_run_opts=(-q -o LogLevel=QUIET -o ServerAliveInterval=2 -o ServerAliveCountMax=1 -o TCPKeepAlive=yes)
	local __arc_tmux_term='xterm-256color'
	local __arc_ssh_cmd="env TERM=${__arc_tmux_term} tmux new-session -A -D -s ${__arc_tmux_session}"
	local __arc_last_err=""

	if ssh "${__arc_ssh_probe_opts[@]}" arc@remotehost true >/dev/null 2>&1; then
		ssh -t "${__arc_ssh_run_opts[@]}" arc@remotehost "$__arc_ssh_cmd"
		__arc_ssh_rc=$?
		(( __arc_ssh_rc != 0 )) && __arc_last_err="remotehost: session attach failed (exit ${__arc_ssh_rc})"
		# Clear the extra terminal line left by ssh/tmux detach return.
		(( __arc_ssh_rc == 0 )) && printf '\r\033[1A\033[2K\r'
		return $__arc_ssh_rc
	else
		__arc_last_err="remotehost: probe failed"
	fi
	if ssh "${__arc_ssh_probe_opts[@]}" arc@pub.remotehost true >/dev/null 2>&1; then
		ssh -t "${__arc_ssh_run_opts[@]}" arc@pub.remotehost "$__arc_ssh_cmd"
		__arc_ssh_rc=$?
		(( __arc_ssh_rc != 0 )) && __arc_last_err="pub.remotehost: session attach failed (exit ${__arc_ssh_rc})"
		(( __arc_ssh_rc == 0 )) && printf '\r\033[1A\033[2K\r'
		return $__arc_ssh_rc
	else
		if [[ -n "$__arc_last_err" ]]; then
			__arc_last_err="${__arc_last_err}; pub.remotehost: probe failed"
		else
			__arc_last_err="pub.remotehost: probe failed"
		fi
	fi
	printf 'sw: cannot connect (%s)\n' "$__arc_last_err" >&2
	printf 'sw: run `ssh -vv arc@remotehost true` and `ssh -vv arc@pub.remotehost true` for details\n' >&2
	return 255
}

# sw: on local, attach/create remote tmux session.
# Usage:
#   sw            -> session "arc"
#   sw <session>  -> named session
sw() {
	__arc_sw_connect "$1"
}

# sl: list remote tmux sessions (same host selection as sw).
sl() {
	local __arc_ssh_probe_opts=(-o BatchMode=yes -o ConnectTimeout=2 -o ConnectionAttempts=1 -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR)
	local __arc_ssh_run_opts=(-q -o LogLevel=QUIET -o ServerAliveInterval=2 -o ServerAliveCountMax=1 -o TCPKeepAlive=yes)
	local __arc_ls_cmd='env TERM=xterm-256color sh -lc '"'"'tmux ls 2>/dev/null || echo "no tmux sessions"'"'"''

	if ssh "${__arc_ssh_probe_opts[@]}" arc@remotehost true >/dev/null 2>&1; then
		ssh "${__arc_ssh_run_opts[@]}" arc@remotehost "$__arc_ls_cmd"
		return $?
	fi
	if ssh "${__arc_ssh_probe_opts[@]}" arc@pub.remotehost true >/dev/null 2>&1; then
		ssh "${__arc_ssh_run_opts[@]}" arc@pub.remotehost "$__arc_ls_cmd"
		return $?
	fi
	printf 'sl: cannot reach remotehost or pub.remotehost\n' >&2
	return 255
}

# x: close remote tmux session.
# Usage:
#   x            -> kill session "arc"
#   x <session>  -> kill named session
x() {
	local __arc_tmux_session="${1:-arc}"
	if [[ ! "$__arc_tmux_session" =~ ^[A-Za-z0-9._-]+$ ]]; then
		printf 'x: invalid session name %q (allowed: letters, digits, ., _, -)\n' "$__arc_tmux_session" >&2
		return 2
	fi

	local __arc_ssh_probe_opts=(-o BatchMode=yes -o ConnectTimeout=2 -o ConnectionAttempts=1 -o StrictHostKeyChecking=accept-new -o LogLevel=ERROR)
	local __arc_ssh_run_opts=(-q -o LogLevel=QUIET -o ServerAliveInterval=2 -o ServerAliveCountMax=1 -o TCPKeepAlive=yes)
	local __arc_x_cmd="env TERM=xterm-256color sh -lc 'tmux kill-session -t ${__arc_tmux_session} 2>/dev/null || { echo \"x: session not found: ${__arc_tmux_session}\" >&2; exit 1; }'"

	if ssh "${__arc_ssh_probe_opts[@]}" arc@remotehost true >/dev/null 2>&1; then
		ssh "${__arc_ssh_run_opts[@]}" arc@remotehost "$__arc_x_cmd"
		return $?
	fi
	if ssh "${__arc_ssh_probe_opts[@]}" arc@pub.remotehost true >/dev/null 2>&1; then
		ssh "${__arc_ssh_run_opts[@]}" arc@pub.remotehost "$__arc_x_cmd"
		return $?
	fi
	printf 'x: cannot reach remotehost or pub.remotehost\n' >&2
	return 255
}

# ARC AUTO SSH (local)
# Attempt arc@remotehost first (WG/LAN), then arc@pub.remotehost (public).
# Never prompt for passwords during auto-connect; if both fail, stay local.
if [[ -z "${ARC_AUTO_SSH_ONCE-}" ]]; then
	ARC_AUTO_SSH_ONCE=1
	if [[ -z "${SSH_CONNECTION-}" ]]; then
		__arc_sw_connect
	fi
fi

__arc_fgc() { printf '\[\e[38;2;%s;%s;%sm\]' "$1" "$2" "$3"; }
__arc_bgc() { printf '\[\e[48;2;%s;%s;%sm\]' "$1" "$2" "$3"; }
__arc_rst='\[\e[0m\]'

__ARC_BG0="$(__arc_bgc 0 0 0)"          # black
__ARC_FG0="$(__arc_fgc 0 0 0)"          # black (foreground)
__ARC_TEXT="$(__arc_fgc 238 238 238)"   # EEEEEE
__ARC_SUB="$(__arc_fgc 136 136 136)"    # 888888
__ARC_LIME="$(__arc_fgc 182 214 0)"     # B6D600 (darker lime)
__ARC_ERR="$(__arc_fgc 255 77 77)"      # FF4D4D

__ARC_CWD_BG="$(__arc_bgc 42 42 42)"     # 2A2A2A (brighter for contrast)
__ARC_GIT_BG="$(__arc_bgc 28 28 28)"     # 1C1C1C (distinct from cwd)
__ARC_ERR_BG="$(__arc_bgc 255 77 77)"    # FF4D4D
__ARC_GLOBE_BG="$(__arc_bgc 182 214 0)"  # B6D600 (darker lime)

__arc_icon_folder() { printf ''; }
__arc_icon_branch() { printf ''; }
__arc_sep() { printf ''; }

__arc_git() {
	git rev-parse --is-inside-work-tree >/dev/null 2>&1 || return 0
	local b dirty
	b="$(git symbolic-ref --quiet --short HEAD 2>/dev/null || git rev-parse --short HEAD 2>/dev/null)" || return 0
	dirty=""
	git diff --quiet --ignore-submodules -- 2>/dev/null || dirty="󰦒"
	printf '%s%s' "$b" "$dirty"
}

__arc_cwd() {
	local p="${PWD/#$HOME/~}"
	local IFS=/ parts out n start
	read -r -a parts <<<"${p#/}"
	n=${#parts[@]}
	if (( n > 3 )); then
		start=$((n-3))
		out="…/${parts[start]}/${parts[start+1]}/${parts[start+2]}"
	else
		out="$p"
	fi
	printf '%s' "$out"
}

__arc_vpn_icon() {
	local __arc_now __arc_hs __arc_recent=0 __arc_ping_ok=0
	__arc_now="$(date +%s)"

	# Recompute at most every 3 seconds to avoid running checks on every prompt render.
	if (( __arc_now - ${__ARC_VPN_CACHE_TS:-0} >= 3 )); then
		__ARC_VPN_CACHE_TS="$__arc_now"
		__ARC_VPN_CACHE_ON=0

		# Hard requirement: route to the server must go via wg0.
		if ip -o route get 10.0.0.1 2>/dev/null | grep -Eq 'dev[[:space:]]+wg0([[:space:]]|$)'; then
			# Fast signal: recent handshake.
			__arc_hs="$(wg show wg0 latest-handshakes 2>/dev/null | awk 'NF>=2 && $2>m{m=$2} END{print m+0}')"
			if (( __arc_hs > 0 && (__arc_now - __arc_hs) <= 180 )); then
				__arc_recent=1
			fi

			# Fallback signal: quick connectivity check.
			if command -v timeout >/dev/null 2>&1; then
				if timeout 0.35 ping -n -c1 -W1 10.0.0.1 >/dev/null 2>&1; then
					__arc_ping_ok=1
				fi
			else
				if ping -n -c1 -W1 10.0.0.1 >/dev/null 2>&1; then
					__arc_ping_ok=1
				fi
			fi

			if (( __arc_recent == 1 || __arc_ping_ok == 1 )); then
				__ARC_VPN_CACHE_ON=1
			fi
		fi
	fi

	if (( ${__ARC_VPN_CACHE_ON:-0} == 1 )); then
		printf '󰓢 '
	fi
}

__arc_update_ps1() {
	local last=$?
	local sep="$(__arc_sep)"
	local folder="$(__arc_icon_folder)"
	local branch_icon="$(__arc_icon_branch)"
	local vpn_icon="$(__arc_vpn_icon)"
	local ps=""
	local g="$(__arc_git)"

	if (( last != 0 )); then
		ps+="${__ARC_ERR_BG}${__ARC_TEXT} ✗ ${last} ${__ARC_BG0}${__ARC_ERR}${sep}${__arc_rst}"
	fi

	ps+="${__ARC_BG0}${__ARC_LIME} ${vpn_icon}󰌢 ${__ARC_CWD_BG}${__ARC_FG0}${sep}${__arc_rst}"

	if [[ -n "$g" ]]; then
		ps+="${__ARC_CWD_BG}${__ARC_LIME} ${folder}  ${__ARC_TEXT}$(__arc_cwd) ${__ARC_GIT_BG}$(__arc_fgc 42 42 42)${sep}${__arc_rst}"
		ps+="${__ARC_GIT_BG}${__ARC_LIME} ${branch_icon} ${__ARC_TEXT}${g} \[\e[49m\]$(__arc_fgc 28 28 28)${sep}${__arc_rst}"
	else
		ps+="${__ARC_CWD_BG}${__ARC_LIME} ${folder}  ${__ARC_TEXT}$(__arc_cwd) \[\e[49m\]$(__arc_fgc 42 42 42)${sep}${__arc_rst}"
	fi

	PS1="${ps} ${__ARC_LIME}❯${__arc_rst} "
}

case ";$PROMPT_COMMAND;" in
	*";__arc_update_ps1;"*) ;;
	"") PROMPT_COMMAND="__arc_update_ps1" ;;
	*) PROMPT_COMMAND="__arc_update_ps1; $PROMPT_COMMAND" ;;
esac
### ARC_PROMPT_END
