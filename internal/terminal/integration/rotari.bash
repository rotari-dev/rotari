# rotari's shell integration for bash. rotari starts bash with --init-file naming this
# file in place of a login shell's startup files, so it first reads those files as a login
# shell would, and then marks prompts and commands with OSC 133 for the Web terminal's
# blocks: D (the last command's end, with its exit status) and A (a prompt) before each
# prompt, B (the command line) at the prompt's end, and C (the output) before a command
# runs. The user's own files are not changed.
if [ -r /etc/profile ]; then
	. /etc/profile
fi
for __rotari_file in "$HOME/.bash_profile" "$HOME/.bash_login" "$HOME/.profile"; do
	if [ -r "$__rotari_file" ]; then
		. "$__rotari_file"
		break
	fi
done
unset __rotari_file

__rotari_status=0
# __rotari_keep runs first before each prompt: it keeps the command's status for the mark,
# and gives it back to what runs after it.
__rotari_keep() {
	__rotari_status=$?
	return "$__rotari_status"
}
# __rotari_prompt runs last. A prompt a theme rebuilt has lost B, which is put back.
__rotari_prompt() {
	printf '\033]133;D;%s\007\033]133;A\007' "$__rotari_status"
	case $PS1 in
	*'133;B'*) ;;
	*) PS1="$PS1"'\[\e]133;B\a\]' ;;
	esac
	return "$__rotari_status"
}
if [[ "$(declare -p PROMPT_COMMAND 2>/dev/null)" == "declare -a"* ]]; then
	PROMPT_COMMAND=(__rotari_keep "${PROMPT_COMMAND[@]}" __rotari_prompt)
else
	# Lines, not semicolons, since the user's may end with one.
	PROMPT_COMMAND=$'__rotari_keep\n'"${PROMPT_COMMAND:-}"$'\n__rotari_prompt'
fi
# Bash prints PS0 before running a command line from 4.4 on; before that, there is no C.
case ${PS0:-} in
*'133;C'*) ;;
*) PS0="${PS0:-}"'\e]133;C\a' ;;
esac
