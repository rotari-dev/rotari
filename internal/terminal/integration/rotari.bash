# rotari's shell integration for bash. rotari starts bash with --init-file naming this
# file in place of a login shell's startup files, so it first reads those files as a login
# shell would, and then marks prompts and commands with OSC 133 for the Web terminal's
# blocks (marks.bash). The user's own files are not changed.
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

# The marks, from marks.bash beside this file.
. "${ROTARI_SHELL_INTEGRATION:-${BASH_SOURCE[0]%/*}}/marks.bash"
