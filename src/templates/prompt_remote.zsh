### ARC_PROMPT_START
# ARC PROMPT (powerline style, theme aligned)
# Requires a font with powerline/nerd glyphs.
[[ -o interactive ]] || return

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

# ARC custom lightweight UX (no external plugins).
autoload -Uz compinit 2>/dev/null || true
if typeset -f compinit >/dev/null 2>&1; then
	compinit -d "$HOME/.zcompdump" 2>/dev/null || true
fi

# History search with Up/Down for current prefix.
bindkey '^[[A' history-beginning-search-backward
bindkey '^[[B' history-beginning-search-forward

__arc_fancy_refresh() {
	RBUFFER=""
	region_highlight=()

	# Show suggestion only when cursor is at end of left buffer.
	[[ $CURSOR -eq ${#LBUFFER} ]] || return
	local prefix="$LBUFFER"
	[[ -n "$prefix" ]] || return

	local h
	for h in ${(f)"$(fc -lnr 1 2>/dev/null)"}; do
		[[ "$h" == "$prefix"* ]] || continue
		[[ "$h" == "$prefix" ]] && continue
		RBUFFER="${h#$prefix}"
		local start=${#LBUFFER}
		local end=$((start + ${#RBUFFER}))
		region_highlight+=("$start $end fg=244")
		return
	done
}

__arc_fancy_self_insert() { zle .self-insert; __arc_fancy_refresh; }
__arc_fancy_backward_delete() { zle .backward-delete-char; __arc_fancy_refresh; }
__arc_fancy_delete_char() { zle .delete-char; __arc_fancy_refresh; }
__arc_fancy_kill_word() { zle .kill-word; __arc_fancy_refresh; }
__arc_fancy_backward_kill_word() { zle .backward-kill-word; __arc_fancy_refresh; }
__arc_fancy_transpose_words() { zle .transpose-words; __arc_fancy_refresh; }
__arc_fancy_accept_line() { RBUFFER=""; region_highlight=(); zle .accept-line; }
__arc_fancy_accept_suggestion_or_move() {
	if [[ -n "$RBUFFER" ]]; then
		LBUFFER+="$RBUFFER"
		RBUFFER=""
		__arc_fancy_refresh
		return
	fi
	zle .forward-char
}
__arc_fancy_redisplay() { __arc_fancy_refresh; zle .redisplay; }
__arc_fancy_line_init() { __arc_fancy_refresh; }

zle -N self-insert __arc_fancy_self_insert
zle -N backward-delete-char __arc_fancy_backward_delete
zle -N delete-char __arc_fancy_delete_char
zle -N kill-word __arc_fancy_kill_word
zle -N backward-kill-word __arc_fancy_backward_kill_word
zle -N transpose-words __arc_fancy_transpose_words
zle -N accept-line __arc_fancy_accept_line
zle -N redisplay __arc_fancy_redisplay
zle -N zle-line-init __arc_fancy_line_init
zle -N forward-char __arc_fancy_accept_suggestion_or_move

__arc_fgc() { printf '%%{\e[38;2;%s;%s;%sm%%}' "$1" "$2" "$3"; }
__arc_bgc() { printf '%%{\e[48;2;%s;%s;%sm%%}' "$1" "$2" "$3"; }
__arc_rst() { printf '%%{\033[0m%%}'; }
__arc_nbg() { printf '%%{\033[49m%%}'; }

__ARC_BG0="$(__arc_bgc 0 0 0)"          # black
__ARC_FG0="$(__arc_fgc 0 0 0)"          # black (foreground)
__ARC_TEXT="$(__arc_fgc 238 238 238)"   # EEEEEE
__ARC_SUB="$(__arc_fgc 136 136 136)"    # 888888
__ARC_LIME="$(__arc_fgc 182 214 0)"     # B6D600 (darker lime)
__ARC_ERR="$(__arc_fgc 255 77 77)"      # FF4D4D
__ARC_RST="$(__arc_rst)"
__ARC_NBG="$(__arc_nbg)"

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
	git diff --quiet --ignore-submodules -- 2>/dev/null || dirty="*"
	printf '%s%s' "$b" "$dirty"
}

__arc_cwd() {
	local p="${PWD/#$HOME/~}"
	local -a parts
	local out n
	parts=("${(@s:/:)${p#/}}")
	n=${#parts[@]}
	if (( n > 3 )); then
		out="…/${parts[n-2]}/${parts[n-1]}/${parts[n]}"
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
		ps+="${__ARC_ERR_BG}${__ARC_TEXT} ✗ ${last} ${__ARC_GLOBE_BG}${__ARC_ERR}${sep}${__ARC_RST}"
	fi

	ps+="${__ARC_GLOBE_BG}${__ARC_FG0} ${vpn_icon}󰖟 ${__ARC_CWD_BG}${__ARC_LIME}${sep}${__ARC_RST}"

	if [[ -n "$g" ]]; then
		ps+="${__ARC_CWD_BG}${__ARC_LIME} ${folder}  ${__ARC_TEXT}$(__arc_cwd) ${__ARC_GIT_BG}$(__arc_fgc 42 42 42)${sep}${__ARC_RST}"
		ps+="${__ARC_GIT_BG}${__ARC_LIME} ${branch_icon} ${__ARC_TEXT}${g} ${__ARC_NBG}$(__arc_fgc 28 28 28)${sep}${__ARC_RST}"
	else
		ps+="${__ARC_CWD_BG}${__ARC_LIME} ${folder}  ${__ARC_TEXT}$(__arc_cwd) ${__ARC_NBG}$(__arc_fgc 42 42 42)${sep}${__ARC_RST}"
	fi

	top+="${__ARC_GIT_BG}${__ARC_LIME} >_ ${__ARC_TEXT}${session_name} ${__ARC_NBG}$(__arc_fgc 28 28 28)${sep_skew}${__ARC_RST}"
	PROMPT="${top}\n${ps} ${__ARC_LIME}❯${__ARC_RST} "
}

setopt prompt_subst
typeset -ga precmd_functions
(( ${precmd_functions[(Ie)__arc_update_ps1]} )) || precmd_functions+=(__arc_update_ps1)
### ARC_PROMPT_END
