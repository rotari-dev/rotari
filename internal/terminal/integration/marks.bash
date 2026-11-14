# rotari's marks for bash: OSC 133 for the Web terminal's blocks, D (the last command's end,
# with its exit status) and A (a prompt) before each prompt, B (the command line) at the
# prompt's end, and C (the output) before a command runs. rotari's own bash reads this
# after the user's files, and a bash started inside one of the panel's shells reads it at
# its first prompt, from the loader in the PROMPT_COMMAND it inherits. It loads once per
# shell, and changes nothing else of the shell but what is said here.
[ -n "${__rotari_marks+x}" ] && return 0
__rotari_marks=1

# __rotari_loader comes first in PROMPT_COMMAND: it keeps the last command's status for the
# mark, and in a shell that has not read this file, reads it. Shells started inside
# inherit it, and zsh ignores it.
__rotari_loader='__rotari_status=$?; [ -n "${__rotari_marks+x}" ] || { [ -r "$ROTARI_SHELL_INTEGRATION/marks.bash" ] && . "$ROTARI_SHELL_INTEGRATION/marks.bash"; }'
__rotari_status=0
# __rotari_keep gives the kept status back to what runs after it.
__rotari_keep() {
	return "$__rotari_status"
}
# __rotari_prompt runs last. A prompt a theme rebuilt has lost B, which is put back. What
# runs next is typed at the prompt, which the DEBUG trap of bash before 4.4 waits for.
__rotari_prompt() {
	__rotari_at_prompt=1
	__rotari_history=$(builtin fc -l -0 2>/dev/null)
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
	local now line url= char hex i
	now=$(builtin fc -l -0 2>/dev/null)
	if [ -z "$now" ] || [ "$now" = "$__rotari_history" ]; then
		printf '\033]133;C\007'
		return
	fi
	line=$(builtin fc -ln -0 2>/dev/null)
	line=${line#$'\t'}
	line=${line# }
	local LC_ALL=C
	for ((i = 0; i < ${#line}; i++)); do
		char=${line:i:1}
		case $char in
		[A-Za-z0-9._~-]) url+=$char ;;
		*)
			# A byte above 127 can read as negative, by the locale.
			printf -v hex '%02X' "'$char"
			url+="%${hex: -2}"
			;;
		esac
	done
	printf '\033]133;C;cmdline_url=%s\007' "$url"
}

# PROMPT_COMMAND is made again from what the user's files left, rotari's pieces taken out
# wherever they are (an inherited copy, a .bashrc that added to it), the loader first,
# __rotari_keep next, and __rotari_prompt last, so that none runs twice; and exported, for a
# bash started inside to run the loader.
if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
	__rotari_commands=()
	for __rotari_command in "${PROMPT_COMMAND[@]}"; do
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
elif [ -z "$(trap -p DEBUG)" ]; then
	# Before 4.4, the DEBUG trap prints C before the first command after a prompt. A line
	# with no command reaches the loader first, which prints nothing. A DEBUG trap of the
	# user's is left alone, and its commands then get no C.
	__rotari_at_prompt=
	__rotari_debug() {
		case $BASH_COMMAND in
		__rotari_status=*) __rotari_at_prompt= ;;
		*)
			if [ -n "$__rotari_at_prompt" ]; then
				__rotari_at_prompt=
				__rotari_c
			fi
			;;
		esac
	}
	trap '__rotari_debug' DEBUG
fi
unset __rotari_old_bash
