# rotari's marks for bash: OSC 133 for the Web terminal's blocks, D (the last command's end,
# with its exit status) and A (a prompt) before each prompt, after OSC 7 (the directory, as
# file://HOST/PATH), B (the command line) at the prompt's end, and C (the output) before a
# command runs. rotari's own bash reads this
# after the user's files, and a bash started inside one of the panel's shells reads it at
# its first prompt, from the loader in the PROMPT_COMMAND it inherits. It loads once per
# shell, and changes nothing else of the shell but what is said here.
[ -n "${__rotari_marks+x}" ] && return 0
__rotari_marks=1

# __rotari_loader comes first in PROMPT_COMMAND: it keeps the last command's status for the
# mark, and in a shell that has not read this file, reads it. Shells started inside
# inherit it, and zsh ignores it.
__rotari_loader='__rotari_status=$?; [ -n "${__rotari_marks+x}" ] || { [ -r "$ROTARI_SHELL_INTEGRATION/marks.bash" ] && { __rotari_loading=1; . "$ROTARI_SHELL_INTEGRATION/marks.bash"; }; }'
__rotari_first_prompt=
# A later user hook could rebuild PS1, so only mark this prompt here when the
# loader is the last command of the inherited prompt hook.
if [ -n "${__rotari_loading:-}" ] && [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" != "declare -a"* ]]; then
	__rotari_initial=${PROMPT_COMMAND:-}
	while [[ $__rotari_initial == *[[:space:]\;] ]]; do
		__rotari_initial=${__rotari_initial%?}
	done
	case $__rotari_initial in
	*"$__rotari_loader")
		__rotari_before=${__rotari_initial%"$__rotari_loader"}
		case $__rotari_before in *"$__rotari_loader"*) ;; *) __rotari_first_prompt=1 ;; esac
		;;
	esac
fi
unset __rotari_before __rotari_initial
if [ -z "${__rotari_loading:-}" ]; then
	__rotari_status=0
fi
# __rotari_keep gives the kept status back to what runs after it.
__rotari_keep() {
	return "$__rotari_status"
}
# __rotari_prompt runs last. A prompt a theme rebuilt has lost B, which is put back. What
# runs next is typed at the prompt, which the DEBUG trap of bash before 4.4 waits for.
__rotari_prompt() {
	__rotari_at_prompt=1
	# Unlike fc -l -0, history 1 includes the newest entry on bash 3.2.
	__rotari_history=$(HISTTIMEFORMAT= builtin history 1 2>/dev/null)
	__rotari_place
	__rotari_dir
	printf '\033]133;D;%s\007\033]133;A\007' "$__rotari_status"
	case $PS1 in
	*'133;B'*) ;;
	*) PS1="$PS1"'\[\e]133;B\a\]' ;;
	esac
	return "$__rotari_status"
}
# __rotari_c prints C, saying the command line as kitty does (C;cmdline_url= and the line
# percent-encoded), since a prompt may redraw the line it was typed on before it runs, as
# a transient prompt does. Bash says the line only to history, so it is said only when
# history has just taken it: a line that history leaves out, by HISTCONTROL or HISTIGNORE,
# would be said as the one before it, and is left to the screen instead.
__rotari_history=
__rotari_c() {
	local now line
	now=$(HISTTIMEFORMAT= builtin history 1 2>/dev/null)
	if [ -z "$now" ] || [ "$now" = "$__rotari_history" ]; then
		printf '\033]133;C\007'
		return
	fi
	line=${now#"${now%%[![:space:]]*}"}
	line=${line#"${line%%[!0-9]*}"}
	line=${line:2}
	__rotari_encode "$line"
	printf '\033]133;C;cmdline_url=%s\007' "$__rotari_url"
}
# __rotari_encode percent-encodes $1 byte by byte into __rotari_url, keeping / as it is
# when $2 is set, for a path.
__rotari_url=
__rotari_encode() {
	local LC_ALL=C char hex i
	__rotari_url=
	for ((i = 0; i < ${#1}; i++)); do
		char=${1:i:1}
		case $char in
		[A-Za-z0-9._~-]) __rotari_url+=$char ;;
		/) if [ -n "${2:-}" ]; then __rotari_url+=/; else __rotari_url+=%2F; fi ;;
		*)
			# A byte above 127 can read as negative, by the locale.
			printf -v hex '%02X' "'$char"
			__rotari_url+="%${hex: -2}"
			;;
		esac
	done
}
# __rotari_dir prints OSC 7, the directory the next command runs in, as file://HOST/PATH
# with the path percent-encoded, so that the Web terminal resolves the relative paths a
# command prints where it ran. It encodes a directory once.
__rotari_dir_said=
__rotari_dir_url=
__rotari_dir() {
	if [ "$PWD" != "$__rotari_dir_said" ] || [ -z "$__rotari_dir_url" ]; then
		__rotari_encode "$PWD" path
		__rotari_dir_url=$__rotari_url
		__rotari_dir_said=$PWD
	fi
	printf '\033]7;file://%s%s\007' "${HOSTNAME:-}" "$__rotari_dir_url"
}
# __rotari_place takes the variables of the place the Web terminal moved this shell to,
# from the file ROTARI_TERMINAL_PLACE names: a line "rotari-place N", then lines
# "set ROTARI_NAME=VALUE" and "unset ROTARI_NAME". It takes each move once, by its N, kept
# in ROTARI_TERMINAL_PLACE_APPLIED for shells started inside too, so that a value set by
# hand afterwards is kept. Values are assigned, never evaluated, and it forks nothing.
__rotari_place() {
	[ -n "${ROTARI_TERMINAL_PLACE:-}" ] && [ -r "$ROTARI_TERMINAL_PLACE" ] || return 0
	local head serial applied line name
	applied=${ROTARI_TERMINAL_PLACE_APPLIED:-0}
	case $applied in '' | *[!0-9]*) applied=0 ;; esac
	{
		IFS= read -r head || return 0
		serial=${head#rotari-place }
		case $serial in '' | *[!0-9]*) return 0 ;; esac
		[ "$serial" -gt "$applied" ] || return 0
		while IFS= read -r line; do
			case $line in
			'set ROTARI_'?*=*)
				line=${line#set }
				name=${line%%=*}
				case $name in *[!A-Z0-9_]*) continue ;; esac
				export "$name=${line#*=}"
				;;
			'unset ROTARI_'?*)
				name=${line#unset }
				case $name in *[!A-Z0-9_]*) continue ;; esac
				unset "$name"
				;;
			esac
		done
		export ROTARI_TERMINAL_PLACE_APPLIED="$serial"
	} <"$ROTARI_TERMINAL_PLACE"
}

# PROMPT_COMMAND is made again from what the user's files left, rotari's pieces taken out
# wherever they are (an inherited copy, a .bashrc that added to it), the loader first,
# __rotari_keep next, and __rotari_prompt last, so that none runs twice; and exported, for a
# bash started inside to run the loader.
if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
	__rotari_commands=()
	for __rotari_command in "${PROMPT_COMMAND[@]}"; do
		__rotari_command=${__rotari_command//"$__rotari_loader;"/}
		__rotari_command=${__rotari_command//"$__rotari_loader"/}
		case $__rotari_command in
		'' | __rotari_keep | __rotari_prompt) ;;
		*) __rotari_commands+=("$__rotari_command") ;;
		esac
	done
	# An array cannot be exported, as systemd's /etc/profile.d makes PROMPT_COMMAND one, so
	# it becomes the same commands on lines of a string, which bash runs alike and a later
	# PROMPT_COMMAND+=(...) adds to as before.
	__rotari_joined=$__rotari_loader$'\n__rotari_keep'
	for __rotari_command in "${__rotari_commands[@]}"; do
		__rotari_joined+=$'\n'$__rotari_command
	done
	unset PROMPT_COMMAND
	PROMPT_COMMAND=$__rotari_joined$'\n__rotari_prompt'
	export PROMPT_COMMAND
	unset __rotari_commands __rotari_command __rotari_joined
else
	__rotari_commands=
	while IFS= read -r __rotari_command; do
		__rotari_command=${__rotari_command//"$__rotari_loader;"/}
		__rotari_command=${__rotari_command//"$__rotari_loader"/}
		case $__rotari_command in
		'' | __rotari_keep | __rotari_prompt | ';') ;;
		*) __rotari_commands+=$'\n'$__rotari_command ;;
		esac
	done <<<"${PROMPT_COMMAND:-}"
	# Lines, not semicolons, since the user's may end with one.
	PROMPT_COMMAND=$__rotari_loader$'\n__rotari_keep'$__rotari_commands$'\n__rotari_prompt'
	# A bash started inside runs it, and through the loader, reads this file.
	export PROMPT_COMMAND
	unset __rotari_commands __rotari_command
fi

# A zsh started anywhere below reads rotari's zsh files, and the user's through them: bash
# ignores ZDOTDIR, so it is exported for the whole session, the user's own in
# ROTARI_USER_ZDOTDIR. One already rotari's, inherited, is left as it is.
if [ -r "${ROTARI_SHELL_INTEGRATION:-}/zsh/.zshenv" ] && [ "${ZDOTDIR:-}" != "$ROTARI_SHELL_INTEGRATION/zsh" ]; then
	if [ -n "${ZDOTDIR+x}" ]; then
		export ROTARI_USER_ZDOTDIR="$ZDOTDIR"
	fi
	export ZDOTDIR="$ROTARI_SHELL_INTEGRATION/zsh"
fi

__rotari_old_bash=
if ((BASH_VERSINFO[0] < 4 || (BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] < 4))) || [ -n "${ROTARI_BASH_DEBUG_TRAP:-}" ]; then
	__rotari_old_bash=1
fi
# ROTARI_BASH_DEBUG_TRAP lets rotari's tests take the old bash's way with a new one.
unset ROTARI_BASH_DEBUG_TRAP
if [ -z "$__rotari_old_bash" ]; then
	# Bash prints PS0 before running a command line from 4.4 on, and expands what it holds
	# there unless promptvars is off.
	case ${PS0:-} in
	*'133;C'* | *__rotari_c*) ;;
	*)
		if shopt -q promptvars; then
			PS0="${PS0:-}"'$(__rotari_c)'
		else
			PS0="${PS0:-}"'\e]133;C\a'
		fi
		;;
	esac
fi
# The DEBUG trap runs before the first command after a prompt: before 4.4 it prints C, and
# in a shell the Web terminal can move, it takes a move made while the shell waited at its
# prompt, so that the command typed next already has the new place, since PS0 runs in a
# subshell that could not. A line with no command reaches the loader first, which does
# neither. A DEBUG trap of the user's is left alone: its commands then get no C, and a move
# waits for the next prompt.
__rotari_debug_c=$__rotari_old_bash
if { [ -n "$__rotari_debug_c" ] || [ -n "${ROTARI_TERMINAL_PLACE:-}" ]; } && [ -z "$(trap -p DEBUG)" ]; then
	__rotari_at_prompt=
	__rotari_debug() {
		case $BASH_COMMAND in
		__rotari_status=*) __rotari_at_prompt= ;;
		*)
			if [ -n "$__rotari_at_prompt" ]; then
				__rotari_at_prompt=
				[ -z "$__rotari_debug_c" ] || __rotari_c
				__rotari_place
			fi
			;;
		esac
		# With extdebug, a trap that fails skips the command.
		return 0
	}
	trap '__rotari_debug' DEBUG
fi
unset __rotari_old_bash __rotari_loading
if [ -n "$__rotari_first_prompt" ]; then
	unset __rotari_first_prompt
	__rotari_prompt
else
	unset __rotari_first_prompt
fi
