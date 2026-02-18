### ARC_PROMPT_START
# ARC PROMPT (powerline style, theme aligned)
# Requires a font with powerline/nerd glyphs.
[[ $- != *i* ]] && return

# sw: on remote, quickly leave the current SSH session.
sw() {
	[[ -n "${SSH_CONNECTION-}" ]] || return 0

	local __arc_tmux_session="${1-}"
	if [[ -n "$__arc_tmux_session" ]]; then
		if [[ ! "$__arc_tmux_session" =~ ^[A-Za-z0-9._-]+$ ]]; then
			printf 'sw: invalid session name %q (allowed: letters, digits, ., _, -)\n' "$__arc_tmux_session" >&2
			return 2
		fi

		# If already inside tmux, switch current client; otherwise attach/create directly.
		if [[ -n "${TMUX-}" ]]; then
			tmux has-session -t "$__arc_tmux_session" 2>/dev/null || tmux new-session -d -s "$__arc_tmux_session"
			tmux switch-client -t "$__arc_tmux_session"
			return $?
		fi
		tmux new-session -A -D -s "$__arc_tmux_session"
		return $?
	fi

	if [[ -n "${TMUX-}" ]]; then
		tmux detach-client
		return $?
	fi

	printf 'sw: not inside tmux; use "sw <session>" to attach/create one\n' >&2
	return 1
}

# sl: list tmux sessions available on this host.
sl() {
	env TERM=xterm-256color sh -lc 'tmux ls 2>/dev/null || echo "no tmux sessions"'
}

# x: close tmux session.
# Usage:
#   x            -> kill current tmux session (when inside tmux)
#   x <session>  -> kill named session
x() {
	local __arc_tmux_session="${1-}"
	if [[ -n "$__arc_tmux_session" ]]; then
		if [[ ! "$__arc_tmux_session" =~ ^[A-Za-z0-9._-]+$ ]]; then
			printf 'x: invalid session name %q (allowed: letters, digits, ., _, -)\n' "$__arc_tmux_session" >&2
			return 2
		fi
		tmux kill-session -t "$__arc_tmux_session" 2>/dev/null || {
			printf 'x: session not found: %s\n' "$__arc_tmux_session" >&2
			return 1
		}
		return 0
	fi

	if [[ -n "${TMUX-}" ]]; then
		tmux kill-session
		return $?
	fi

	printf 'x: not inside tmux; use "x <session>"\n' >&2
	return 1
}

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
__arc_sep_skew() { printf ''; }
__arc_session_name() {
	if [[ -n "${TMUX-}" ]]; then
		tmux display-message -p '#S' 2>/dev/null && return
	fi
	printf 'shell'
}

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
	local __arc_src_ip
	# On remote, mark VPN only when SSH client source is from WG range.
	if [[ -n "${SSH_CONNECTION-}" ]]; then
		__arc_src_ip="${SSH_CONNECTION%% *}"
		if [[ "$__arc_src_ip" == 10.0.0.* ]]; then
			printf '󰓢 '
			return
		fi
	fi
}

__arc_update_ps1() {
	local last=$?
	local sep="$(__arc_sep)"
	local folder="$(__arc_icon_folder)"
	local branch_icon="$(__arc_icon_branch)"
	local vpn_icon="$(__arc_vpn_icon)"
	local session_name="$(__arc_session_name)"
	local ps=""
	local g="$(__arc_git)"
	local top=""
	local sep_skew="$(__arc_sep_skew)"

	if (( last != 0 )); then
		ps+="${__ARC_ERR_BG}${__ARC_TEXT} ✗ ${last} ${__ARC_GLOBE_BG}${__ARC_ERR}${sep}${__arc_rst}"
	fi

	ps+="${__ARC_GLOBE_BG}${__ARC_FG0} ${vpn_icon}󰖟 ${__ARC_CWD_BG}${__ARC_LIME}${sep}${__arc_rst}"

	if [[ -n "$g" ]]; then
		ps+="${__ARC_CWD_BG}${__ARC_LIME} ${folder}  ${__ARC_TEXT}$(__arc_cwd) ${__ARC_GIT_BG}$(__arc_fgc 42 42 42)${sep}${__arc_rst}"
		ps+="${__ARC_GIT_BG}${__ARC_LIME} ${branch_icon} ${__ARC_TEXT}${g} \[\e[49m\]$(__arc_fgc 28 28 28)${sep}${__arc_rst}"
	else
		ps+="${__ARC_CWD_BG}${__ARC_LIME} ${folder}  ${__ARC_TEXT}$(__arc_cwd) \[\e[49m\]$(__arc_fgc 42 42 42)${sep}${__arc_rst}"
	fi

	top+="${__ARC_GIT_BG}${__ARC_LIME} >_ ${__ARC_TEXT}${session_name} \[\e[49m\]$(__arc_fgc 28 28 28)${sep_skew}${__arc_rst}"
	PS1="${top}\n${ps} ${__ARC_LIME}❯${__arc_rst} "
}

case ";$PROMPT_COMMAND;" in
	*";__arc_update_ps1;"*) ;;
	"") PROMPT_COMMAND="__arc_update_ps1" ;;
	*) PROMPT_COMMAND="__arc_update_ps1; $PROMPT_COMMAND" ;;
esac
### ARC_PROMPT_END
