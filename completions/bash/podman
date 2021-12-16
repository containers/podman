# bash completion V2 for podman                               -*- shell-script -*-

__podman_debug()
{
    if [[ -n ${BASH_COMP_DEBUG_FILE:-} ]]; then
        echo "$*" >> "${BASH_COMP_DEBUG_FILE}"
    fi
}

# Macs have bash3 for which the bash-completion package doesn't include
# _init_completion. This is a minimal version of that function.
__podman_init_completion()
{
    COMPREPLY=()
    _get_comp_words_by_ref "$@" cur prev words cword
}

# This function calls the podman program to obtain the completion
# results and the directive.  It fills the 'out' and 'directive' vars.
__podman_get_completion_results() {
    local requestComp lastParam lastChar args

    # Prepare the command to request completions for the program.
    # Calling ${words[0]} instead of directly podman allows to handle aliases
    args=("${words[@]:1}")
    requestComp="${words[0]} __complete ${args[*]}"

    lastParam=${words[$((${#words[@]}-1))]}
    lastChar=${lastParam:$((${#lastParam}-1)):1}
    __podman_debug "lastParam ${lastParam}, lastChar ${lastChar}"

    if [ -z "${cur}" ] && [ "${lastChar}" != "=" ]; then
        # If the last parameter is complete (there is a space following it)
        # We add an extra empty parameter so we can indicate this to the go method.
        __podman_debug "Adding extra empty parameter"
        requestComp="${requestComp} ''"
    fi

    # When completing a flag with an = (e.g., podman -n=<TAB>)
    # bash focuses on the part after the =, so we need to remove
    # the flag part from $cur
    if [[ "${cur}" == -*=* ]]; then
        cur="${cur#*=}"
    fi

    __podman_debug "Calling ${requestComp}"
    # Use eval to handle any environment variables and such
    out=$(eval "${requestComp}" 2>/dev/null)

    # Extract the directive integer at the very end of the output following a colon (:)
    directive=${out##*:}
    # Remove the directive
    out=${out%:*}
    if [ "${directive}" = "${out}" ]; then
        # There is not directive specified
        directive=0
    fi
    __podman_debug "The completion directive is: ${directive}"
    __podman_debug "The completions are: ${out[*]}"
}

__podman_process_completion_results() {
    local shellCompDirectiveError=1
    local shellCompDirectiveNoSpace=2
    local shellCompDirectiveNoFileComp=4
    local shellCompDirectiveFilterFileExt=8
    local shellCompDirectiveFilterDirs=16

    if [ $((directive & shellCompDirectiveError)) -ne 0 ]; then
        # Error code.  No completion.
        __podman_debug "Received error from custom completion go code"
        return
    else
        if [ $((directive & shellCompDirectiveNoSpace)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __podman_debug "Activating no space"
                compopt -o nospace
            else
                __podman_debug "No space directive not supported in this version of bash"
            fi
        fi
        if [ $((directive & shellCompDirectiveNoFileComp)) -ne 0 ]; then
            if [[ $(type -t compopt) = "builtin" ]]; then
                __podman_debug "Activating no file completion"
                compopt +o default
            else
                __podman_debug "No file completion directive not supported in this version of bash"
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
        __podman_debug "File filtering command: $filteringCmd"
        $filteringCmd
    elif [ $((directive & shellCompDirectiveFilterDirs)) -ne 0 ]; then
        # File completion for directories only

        # Use printf to strip any trailing newline
        local subdir
        subdir=$(printf "%s" "${out[0]}")
        if [ -n "$subdir" ]; then
            __podman_debug "Listing directories in $subdir"
            pushd "$subdir" >/dev/null 2>&1 && _filedir -d && popd >/dev/null 2>&1 || return
        else
            __podman_debug "Listing directories in ."
            _filedir -d
        fi
    else
        __podman_handle_completion_types
    fi

    __podman_handle_special_char "$cur" :
    __podman_handle_special_char "$cur" =
}

__podman_handle_completion_types() {
    __podman_debug "__podman_handle_completion_types: COMP_TYPE is $COMP_TYPE"

    case $COMP_TYPE in
    37|42)
        # Type: menu-complete/menu-complete-backward and insert-completions
        # If the user requested inserting one completion at a time, or all
        # completions at once on the command-line we must remove the descriptions.
        # https://github.com/spf13/cobra/issues/1508
        local tab comp
        tab=$(printf '\t')
        while IFS='' read -r comp; do
            # Strip any description
            comp=${comp%%$tab*}
            # Only consider the completions that match
            comp=$(compgen -W "$comp" -- "$cur")
            if [ -n "$comp" ]; then
                COMPREPLY+=("$comp")
            fi
        done < <(printf "%s\n" "${out[@]}")
        ;;

    *)
        # Type: complete (normal completion)
        __podman_handle_standard_completion_case
        ;;
    esac
}

__podman_handle_standard_completion_case() {
    local tab comp
    tab=$(printf '\t')

    local longest=0
    # Look for the longest completion so that we can format things nicely
    while IFS='' read -r comp; do
        # Strip any description before checking the length
        comp=${comp%%$tab*}
        # Only consider the completions that match
        comp=$(compgen -W "$comp" -- "$cur")
        if ((${#comp}>longest)); then
            longest=${#comp}
        fi
    done < <(printf "%s\n" "${out[@]}")

    local completions=()
    while IFS='' read -r comp; do
        if [ -z "$comp" ]; then
            continue
        fi

        __podman_debug "Original comp: $comp"
        comp="$(__podman_format_comp_descriptions "$comp" "$longest")"
        __podman_debug "Final comp: $comp"
        completions+=("$comp")
    done < <(printf "%s\n" "${out[@]}")

    while IFS='' read -r comp; do
        COMPREPLY+=("$comp")
    done < <(compgen -W "${completions[*]}" -- "$cur")

    # If there is a single completion left, remove the description text
    if [ ${#COMPREPLY[*]} -eq 1 ]; then
        __podman_debug "COMPREPLY[0]: ${COMPREPLY[0]}"
        comp="${COMPREPLY[0]%% *}"
        __podman_debug "Removed description from single completion, which is now: ${comp}"
        COMPREPLY=()
        COMPREPLY+=("$comp")
    fi
}

__podman_handle_special_char()
{
    local comp="$1"
    local char=$2
    if [[ "$comp" == *${char}* && "$COMP_WORDBREAKS" == *${char}* ]]; then
        local word=${comp%"${comp##*${char}}"}
        local idx=${#COMPREPLY[*]}
        while [[ $((--idx)) -ge 0 ]]; do
            COMPREPLY[$idx]=${COMPREPLY[$idx]#"$word"}
        done
    fi
}

__podman_format_comp_descriptions()
{
    local tab
    tab=$(printf '\t')
    local comp="$1"
    local longest=$2

    # Properly format the description string which follows a tab character if there is one
    if [[ "$comp" == *$tab* ]]; then
        desc=${comp#*$tab}
        comp=${comp%%$tab*}

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
    printf "%q" "${comp}"
}

__start_podman()
{
    local cur prev words cword split

    COMPREPLY=()

    # Call _init_completion from the bash-completion package
    # to prepare the arguments properly
    if declare -F _init_completion >/dev/null 2>&1; then
        _init_completion -n "=:" || return
    else
        __podman_init_completion -n "=:" || return
    fi

    __podman_debug
    __podman_debug "========= starting completion logic =========="
    __podman_debug "cur is ${cur}, words[*] is ${words[*]}, #words[@] is ${#words[@]}, cword is $cword"

    # The user could have moved the cursor backwards on the command-line.
    # We need to trigger completion from the $cword location, so we need
    # to truncate the command-line ($words) up to the $cword location.
    words=("${words[@]:0:$cword+1}")
    __podman_debug "Truncated words[*]: ${words[*]},"

    local out directive
    __podman_get_completion_results
    __podman_process_completion_results
}

if [[ $(type -t compopt) = "builtin" ]]; then
    complete -o default -F __start_podman podman
else
    complete -o default -o nospace -F __start_podman podman
fi

# ex: ts=4 sw=4 et filetype=sh

# This file is generated with "podman completion"; see: podman-completion(1)
