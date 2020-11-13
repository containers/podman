package cobra

import (
	"bytes"
	"fmt"
	"io"
	"os"
)

// Annotations for Bash completion.
const (
	BashCompFilenameExt = "cobra_annotation_bash_completion_filename_extensions"
	// BashCompCustom should be avoided as it only works for bash.
	// Function RegisterFlagCompletionFunc() should be used instead.
	BashCompCustom          = "cobra_annotation_bash_completion_custom"
	BashCompOneRequiredFlag = "cobra_annotation_bash_completion_one_required_flag"
	BashCompSubdirsInDir    = "cobra_annotation_bash_completion_subdirs_in_dir"
)

// GenBashCompletion generates bash completion file and writes to the passed writer.
func (c *Command) GenBashCompletion(w io.Writer) error {
	return c.genBashCompletion(w, false)
}

// GenBashCompletionWithDesc generates bash completion file with descriptions and writes to the passed writer.
func (c *Command) GenBashCompletionWithDesc(w io.Writer) error {
	return c.genBashCompletion(w, true)
}

// GenBashCompletionFile generates bash completion file.
func (c *Command) GenBashCompletionFile(filename string) error {
	return c.genBashCompletionFile(filename, false)
}

// GenBashCompletionFileWithDesc generates bash completion file with descriptions.
func (c *Command) GenBashCompletionFileWithDesc(filename string) error {
	return c.genBashCompletionFile(filename, true)
}

func (c *Command) genBashCompletionFile(filename string, includeDesc bool) error {
	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return c.genBashCompletion(outFile, includeDesc)
}

func (c *Command) genBashCompletion(w io.Writer, includeDesc bool) error {
	buf := new(bytes.Buffer)
	if len(c.BashCompletionFunction) > 0 {
		buf.WriteString(c.BashCompletionFunction + "\n")
	}
	genBashComp(buf, c.Name(), includeDesc)

	_, err := buf.WriteTo(w)
	return err
}

func genBashComp(buf *bytes.Buffer, name string, includeDesc bool) {
	compCmd := ShellCompRequestCmd
	if !includeDesc {
		compCmd = ShellCompNoDescRequestCmd
	}

	buf.WriteString(fmt.Sprintf(`# bash completion for %-36[1]s -*- shell-script -*-

__%[1]s_debug()
{
    if [[ -n ${BASH_COMP_DEBUG_FILE} ]]; then
        echo "$*" >> "${BASH_COMP_DEBUG_FILE}"
    fi
}

__%[1]s_perform_completion()
{
    __%[1]s_debug
    __%[1]s_debug "========= starting completion logic =========="
    __%[1]s_debug "cur is ${cur}, words[*] is ${words[*]}, #words[@] is ${#words[@]}, cword is $cword"

    # The user could have moved the cursor backwards on the command-line.
    # We need to trigger completion from the $cword location, so we need
    # to truncate the command-line ($words) up to the $cword location.
    words=("${words[@]:0:$cword+1}")
    __%[1]s_debug "Truncated words[*]: ${words[*]},"

    local shellCompDirectiveError=%[3]d
    local shellCompDirectiveNoSpace=%[4]d
    local shellCompDirectiveNoFileComp=%[5]d
    local shellCompDirectiveFilterFileExt=%[6]d
    local shellCompDirectiveFilterDirs=%[7]d
    local shellCompDirectiveLegacyCustomComp=%[8]d
    local shellCompDirectiveLegacyCustomArgsComp=%[9]d

    local out requestComp lastParam lastChar comp directive args flagPrefix

    # Prepare the command to request completions for the program.
    # Calling ${words[0]} instead of directly %[1]s allows to handle aliases
    args=("${words[@]:1}")
    requestComp="${words[0]} %[2]s ${args[*]}"

    lastParam=${words[$((${#words[@]}-1))]}
    lastChar=${lastParam:$((${#lastParam}-1)):1}
    __%[1]s_debug "lastParam ${lastParam}, lastChar ${lastChar}"

    if [ -z "${cur}" ] && [ "${lastChar}" != "=" ]; then
        # If the last parameter is complete (there is a space following it)
        # We add an extra empty parameter so we can indicate this to the go method.
        __%[1]s_debug "Adding extra empty parameter"
        requestComp="${requestComp} \"\""
    fi

    # When completing a flag with an = (e.g., %[1]s -n=<TAB>)
    # bash focuses on the part after the =, so we need to remove
    # the flag part from $cur
    if [[ "${cur}" == -*=* ]]; then
        flagPrefix="${cur%%%%=*}="
        cur="${cur#*=}"
    fi

    __%[1]s_debug "Calling ${requestComp}"
    # Use eval to handle any environment variables and such
    out=$(eval "${requestComp}" 2>/dev/null)

    # Extract the directive integer at the very end of the output following a colon (:)
    directive=${out##*:}
    # Remove the directive
    out=${out%%:*}
    if [ "${directive}" = "${out}" ]; then
        # There is not directive specified
        directive=0
    fi
    __%[1]s_debug "The completion directive is: ${directive}"
    __%[1]s_debug "The completions are: ${out[*]}"

    if [ $((directive & shellCompDirectiveError)) -ne 0 ]; then
        # Error code.  No completion.
        __%[1]s_debug "Received error from custom completion go code"
        return
    else
        if [ $((directive & shellCompDirectiveNoSpace)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __%[1]s_debug "Activating no space"
                compopt -o nospace
            fi
        fi
        if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __%[1]s_debug "Activating no file completion"
                compopt +o default
            fi
        fi
    fi

    if [ $((directive & shellCompDirectiveFilterFileExt)) -ne 0 ]; then
        # File extension filtering
        local fullFilter filter filteringCmd

        # Do not use quotes around the $out variable or else newline
        # characters will be kept.
        for filter in ${out[*]}; do
            fullFilter+="$filter|"
        done

        filteringCmd="_filedir $fullFilter"
        __%[1]s_debug "File filtering command: $filteringCmd"
        $filteringCmd
    elif [ $((directive & shellCompDirectiveFilterDirs)) -ne 0 ]; then
        # File completion for directories only

        # Use printf to strip any trailing newline
        local subdir
        subdir=$(printf "%%s" "${out[0]}")
        if [ -n "$subdir" ]; then
            __%[1]s_debug "Listing directories in $subdir"
            pushd "$subdir" >/dev/null 2>&1 && _filedir -d && popd >/dev/null 2>&1 || return
        else
            __%[1]s_debug "Listing directories in ."
            _filedir -d
        fi
    elif [ $((directive & shellCompDirectiveLegacyCustomComp)) -ne 0 ]; then
        local cmd
        __%[1]s_debug "Legacy custom completion. Directive: $directive, cmds: ${out[*]}"

        # The following variables should get their value through the commands
        # we have received as completions and are parsing below.
        local last_command
        local nouns

        # Execute every command received
        while IFS='' read -r cmd; do
            __%[1]s_debug "About to execute: $cmd"
            eval "$cmd"
        done < <(printf "%%s\n" "${out[@]}")

        __%[1]s_debug "last_command: $last_command"
        __%[1]s_debug "nouns[0]: ${nouns[0]}, nouns[1]: ${nouns[1]}"

        if [ $((directive & shellCompDirectiveLegacyCustomArgsComp)) -ne 0 ]; then
            # We should call the global legacy custom completion function, if it is defined
            if declare -F __%[1]s_custom_func >/dev/null; then
                # Use command name qualified legacy custom func
                __%[1]s_debug "About to call: __%[1]s_custom_func"
                __%[1]s_custom_func
            elif declare -F __custom_func >/dev/null; then
                # Otherwise fall back to unqualified legacy custom func for compatibility
                __%[1]s_debug "About to call: __custom_func"
                 __custom_func
            fi
        fi
    else
        local tab
        tab=$(printf '\t')
        local longest=0
        # Look for the longest completion so that we can format things nicely
        while IFS='' read -r comp; do
            comp=${comp%%%%$tab*}
            if ((${#comp}>longest)); then
                longest=${#comp}
            fi
        done < <(printf "%%s\n" "${out[@]}")

        local completions=()
        while IFS='' read -r comp; do
            if [ -z "$comp" ]; then
                continue
            fi

            __%[1]s_debug "Original comp: $comp"
            comp="$(__%[1]s_format_comp_descriptions "$comp" "$longest")"
            __%[1]s_debug "Final comp: $comp"
            completions+=("$comp")
        done < <(printf "%%s\n" "${out[@]}")

        while IFS='' read -r comp; do
            # Although this script should only be used for bash
            # there may be programs that still convert the bash
            # script into a zsh one.  To continue supporting those
            # programs, we do this single adaptation for zsh
            if [ -n "${ZSH_VERSION}" ]; then
                # zsh completion needs --flag= prefix
                COMPREPLY+=("$flagPrefix$comp")
            else
                COMPREPLY+=("$comp")
            fi
        done < <(compgen -W "${completions[*]}" -- "$cur")

        # If there is a single completion left, remove the description text
        if [ ${#COMPREPLY[*]} -eq 1 ]; then
            __%[1]s_debug "COMPREPLY[0]: ${COMPREPLY[0]}"
            comp="${COMPREPLY[0]%%%% *}"
            __%[1]s_debug "Removed description from single completion, which is now: ${comp}"
            COMPREPLY=()
            COMPREPLY+=("$comp")
        fi
    fi

    __%[1]s_handle_special_char "$cur" :
    __%[1]s_handle_special_char "$cur" =
}

__%[1]s_handle_special_char()
{
    local comp="$1"
    local char=$2
    if [[ "$comp" == *${char}* && "$COMP_WORDBREAKS" == *${char}* ]]; then
        local word=${comp%%"${comp##*${char}}"}
        local idx=${#COMPREPLY[*]}
        while [[ $((--idx)) -ge 0 ]]; do
            COMPREPLY[$idx]=${COMPREPLY[$idx]#"$word"}
        done
    fi
}

__%[1]s_format_comp_descriptions()
{
    local tab
    tab=$(printf '\t')
    local comp="$1"
    local longest=$2

    # Properly format the description string which follows a tab character if there is one
    if [[ "$comp" == *$tab* ]]; then
        desc=${comp#*$tab}
        comp=${comp%%%%$tab*}

        # $COLUMNS stores the current shell width.
        # Remove an extra 4 because we add 2 spaces and 2 parentheses.
        maxdesclength=$(( COLUMNS - longest - 4 ))

        # Make sure we can fit a description of at least 8 characters
        # if we are to align the descriptions.
        if [[ $maxdesclength -gt 8 ]]; then
            # Add the proper number of spaces to align the descriptions
            for ((i = ${#comp} ; i < longest ; i++)); do
                comp+=" "
            done
        else
            # Don't pad the descriptions so we can fit more text after the completion
            maxdesclength=$(( COLUMNS - ${#comp} - 4 ))
        fi

        # If there is enough space for any description text,
        # truncate the descriptions that are too long for the shell width
        if [ $maxdesclength -gt 0 ]; then
            if [ ${#desc} -gt $maxdesclength ]; then
                desc=${desc:0:$(( maxdesclength - 1 ))}
                desc+="…"
            fi
            comp+="  ($desc)"
        fi
    fi

    # Must use printf to escape all special characters
    printf "%%q" "${comp}"
}

__start_%[1]s()
{
    local cur prev words cword

    COMPREPLY=()
    _get_comp_words_by_ref -n "=:" cur prev words cword

    __%[1]s_perform_completion
}

if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_%[1]s %[1]s
else
    complete -o default -o nospace -F __start_%[1]s %[1]s
fi

# ex: ts=4 sw=4 et filetype=sh
`, name, compCmd,
		ShellCompDirectiveError, ShellCompDirectiveNoSpace, ShellCompDirectiveNoFileComp,
		ShellCompDirectiveFilterFileExt, ShellCompDirectiveFilterDirs,
		shellCompDirectiveLegacyCustomComp, shellCompDirectiveLegacyCustomArgsComp))
}
