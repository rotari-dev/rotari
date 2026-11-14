# rotari's marks for zsh: OSC 133 for the Web terminal's blocks, D (the last command's end,
# with its exit status) and A (a prompt) before each prompt, B (the command line) when the
# line editor starts, and C (the output, saying the command line) before a command runs;
# the cursor's place in the line before each redraw; and the widget for a click. rotari's
# own zsh reads this after the user's .zshrc, and a zsh started inside one of the panel's
# shells does too, through rotari's ZDOTDIR. It loads once per shell.
(( ${+__rotari_marks} )) && return 0
typeset -g __rotari_marks=1

__rotari_status=0
# __rotari_keep runs first before each prompt: it keeps the command's status for the mark,
# and gives it back to what runs after it.
__rotari_keep() {
	__rotari_status=$?
	__rotari_zdotdir_back
	return $__rotari_status
}
# __rotari_prompt runs last, unless a hook is added later, as some themes add theirs at the
# first prompt.
__rotari_prompt() {
	print -n "\e]133;D;${__rotari_status}\a\e]133;A${__rotari_goto_said}\a"
	(( __rotari_line_init )) || [[ $PS1 == *'133;B'* ]] || PS1="$PS1"$'%{\e]133;B\a%}'
	return $__rotari_status
}
# B is printed once the line editor has drawn the prompt, where the command line starts,
# so that a theme that rebuilds the prompt, at any time, cannot take it away. A zsh before
# 5.3, without add-zle-hook-widget, has B put at the end of the prompt instead.
__rotari_line_init=0
__rotari_b() {
	print -n "\e]133;B\a"
}
# __rotari_cursor says, before each redraw of the line, where the cursor is in it, so that
# the Web terminal finds where the line starts from the cursor even after the line editor
# drew the prompt again elsewhere, as Ctrl+L and a list of completions do, without a B.
__rotari_cursor() {
	print -n "\e]133;rotari_cursor=$CURSOR\a"
}
# add-zle-hook-widget does nothing until the line editor is loaded, which an interactive
# zsh does only after its startup files unless asked to.
if zmodload zsh/zle 2>/dev/null && autoload -Uz add-zle-hook-widget 2>/dev/null &&
	add-zle-hook-widget line-init __rotari_b 2>/dev/null; then
	__rotari_line_init=1
	add-zle-hook-widget line-pre-redraw __rotari_cursor 2>/dev/null
fi
# __rotari_preexec says the command line too, as kitty does (C;cmdline_url= and the line
# percent-encoded), since a prompt may redraw the line it was typed on before it runs, as
# powerlevel10k's transient prompt does, leaving nothing between B and C on the screen.
__rotari_preexec() {
	emulate -L zsh
	setopt no_multibyte
	local text=${1:-$3} url= char hex
	local -i i
	for ((i = 1; i <= ${#text}; i++)); do
		char=${text[i]}
		if [[ $char == [A-Za-z0-9._~-] ]]; then
			url+=$char
		else
			# A byte above 127 can read as negative, by the locale.
			printf -v hex '%%%02X' $((#char & 255))
			url+=$hex
		fi
	done
	print -n "\e]133;C;cmdline_url=$url\a"
}
# __rotari_goto puts the cursor where the Web terminal was clicked: the panel sends a key
# sequence no terminal does, the line's character index, and ";". CURSOR stops at the
# buffer's end, so a suggestion shown past it, as zsh-autosuggestions shows one, is not
# taken. The prompt's A says that it can be used.
__rotari_goto() {
	local key index=
	while read -k 1 key && [[ $key == [0-9] && ${#index} -lt 10 ]]; do
		index+=$key
	done
	[[ $key == ';' && -n $index ]] || return 0
	CURSOR=$((index > ${#BUFFER} ? ${#BUFFER} : index))
}
__rotari_goto_said=
if zle -N __rotari_goto 2>/dev/null; then
	for __rotari_keymap in emacs viins vicmd; do
		bindkey -M $__rotari_keymap '\e[9998~' __rotari_goto
	done
	unset __rotari_keymap
	__rotari_goto_said=';rotari_goto=1'
fi
# A zsh started inside this one reads rotari's files, and the user's through them, when
# ZDOTDIR names rotari's: it is exported for a command line whose first word is zsh, and
# only then, since a command line that expands $ZDOTDIR is to see the user's; precmd puts
# the user's back.
__rotari_zdotdir_saved=()
__rotari_zsh_inside() {
	[[ -r ${ROTARI_SHELL_INTEGRATION:-}/zsh/.zshenv ]] || return 0
	local -a words=(${(z)1})
	while (( $#words )) && [[ $words[1] == (*=*|exec|command|builtin|nocorrect|noglob|-) ]]; do
		shift words
	done
	[[ $words[1] == zsh || $words[1] == */zsh ]] || return 0
	__rotari_zdotdir_saved=(${+ZDOTDIR} "${ZDOTDIR:-}")
	if (( ${+ZDOTDIR} )) && [[ $ZDOTDIR != $ROTARI_SHELL_INTEGRATION/zsh ]]; then
		export ROTARI_USER_ZDOTDIR=$ZDOTDIR
	fi
	export ZDOTDIR=$ROTARI_SHELL_INTEGRATION/zsh
}
__rotari_zdotdir_back() {
	(( $#__rotari_zdotdir_saved )) || return 0
	if (( __rotari_zdotdir_saved[1] )); then
		ZDOTDIR=$__rotari_zdotdir_saved[2]
	else
		unset ZDOTDIR
	fi
	unset ROTARI_USER_ZDOTDIR
	__rotari_zdotdir_saved=()
}
# A bash started inside runs the PROMPT_COMMAND it inherits, which zsh ignores: the loader
# of rotari's marks for bash.
if [[ -r ${ROTARI_SHELL_INTEGRATION:-}/marks.bash ]]; then
	export PROMPT_COMMAND='__rotari_status=$?; [ -n "${__rotari_marks+x}" ] || { [ -r "$ROTARI_SHELL_INTEGRATION/marks.bash" ] && { __rotari_loading=1; . "$ROTARI_SHELL_INTEGRATION/marks.bash"; }; }'
fi
precmd_functions=(__rotari_keep $precmd_functions __rotari_prompt)
preexec_functions+=(__rotari_preexec __rotari_zsh_inside)
