### ARC_PROMPT_START
# ARC PROMPT (powerline style, theme aligned)
# Requires a font with powerline/nerd glyphs.
[[ $- != *i* ]] && return

# sw: on remote, quickly leave the current SSH session.
sw() {
	[[ -n "${SSH_CONNECTION-}" ]] && exit
}

__arc_fgc() { printf '\[\e[38;2;%s;%s;%sm\]' "$1" "$2" "$3"; }
__arc_bgc() { printf '\[\e[48;2;%s;%s;%sm\]' "$1" "$2" "$3"; }
__arc_fgc8() { printf '\[\e[3%sm\]' "$1"; }
__arc_bgc8() { printf '\[\e[4%sm\]' "$1"; }
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

__arc_in_screen() {
	[[ "${TERM-}" == screen* || -n "${STY-}" ]]
}

__arc_icon_folder() { printf ''; }
__arc_icon_branch() { printf ''; }
__arc_sep() { printf ''; }

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
	if __arc_in_screen; then
		__arc_update_ps1_screen "$last"
		return
	fi

	local sep="$(__arc_sep)"
	local folder="$(__arc_icon_folder)"
	local branch_icon="$(__arc_icon_branch)"
	local vpn_icon="$(__arc_vpn_icon)"
	local ps=""
	local g="$(__arc_git)"

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

	PS1="${ps} ${__ARC_LIME}❯${__arc_rst} "
}

__arc_update_ps1_screen() {
	local last="$1"
	local g="$(__arc_git)"
	local cwd="$(__arc_cwd)"

	local c_ok="$(__arc_fgc8 2)"   # green
	local c_warn="$(__arc_fgc8 3)" # yellow
	local c_err="$(__arc_fgc8 1)"  # red
	local c_dim="$(__arc_fgc8 7)"  # white/gray
	local c_rst="$__arc_rst"
	local vpn=""
	if [[ -n "$(__arc_vpn_icon)" ]]; then
		vpn="vpn "
	fi

	local ps=""
	if (( last != 0 )); then
		ps+="${c_err}[${last}]${c_rst} "
	fi
	ps+="${c_ok}${vpn}arc${c_rst} ${c_dim}${cwd}${c_rst}"
	if [[ -n "$g" ]]; then
		ps+=" ${c_warn}(git:${g})${c_rst}"
	fi
	PS1="${ps} ${c_ok}>${c_rst} "
}

case ";$PROMPT_COMMAND;" in
	*";__arc_update_ps1;"*) ;;
	"") PROMPT_COMMAND="__arc_update_ps1" ;;
	*) PROMPT_COMMAND="__arc_update_ps1; $PROMPT_COMMAND" ;;
esac
### ARC_PROMPT_END
