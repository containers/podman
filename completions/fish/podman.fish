# fish completion for podman                               -*- shell-script -*-

function __podman_debug
    set file "$BASH_COMP_DEBUG_FILE"
    if test -n "$file"
        echo "$argv" >> $file
    end
end

function __podman_perform_completion
    __podman_debug "Starting __podman_perform_completion"

    set args (string split -- " " (string trim -l (commandline -c)))
    set lastArg "$args[-1]"

    __podman_debug "args: $args"
    __podman_debug "last arg: $lastArg"

    set emptyArg ""
    if test -z "$lastArg"
        __podman_debug "Setting emptyArg"
        set emptyArg \"\"
    end
    __podman_debug "emptyArg: $emptyArg"

    set requestComp "$args[1] __complete $args[2..-1] $emptyArg"
    __podman_debug "Calling $requestComp"

    # Call the command as a sub-shell so that we can redirect any errors
    # For example, if $requestComp has an unmatched quote
    # https://github.com/spf13/cobra/issues/1214
    set results (fish -c "$requestComp" 2> /dev/null)
    set comps $results[1..-2]
    set directiveLine $results[-1]

    # For Fish, when completing a flag with an = (e.g., <program> -n=<TAB>)
    # completions must be prefixed with the flag
    set flagPrefix (string match -r -- '-.*=' "$lastArg")

    __podman_debug "Comps: $comps"
    __podman_debug "DirectiveLine: $directiveLine"
    __podman_debug "flagPrefix: $flagPrefix"

    for comp in $comps
        printf "%s%s\n" "$flagPrefix" "$comp"
    end

    printf "%s\n" "$directiveLine"
end

# This function does two things:
# - Obtain the completions and store them in the global __podman_comp_results
# - Return false if file completion should be performed
function __podman_prepare_completions
    __podman_debug ""
    __podman_debug "========= starting completion logic =========="

    # Start fresh
    set --erase __podman_comp_results

    set results (__podman_perform_completion)
    __podman_debug "Completion results: $results"

    if test -z "$results"
        __podman_debug "No completion, probably due to a failure"
        # Might as well do file completion, in case it helps
        return 1
    end

    set directive (string sub --start 2 $results[-1])
    set --global __podman_comp_results $results[1..-2]

    __podman_debug "Completions are: $__podman_comp_results"
    __podman_debug "Directive is: $directive"

    set shellCompDirectiveError 1
    set shellCompDirectiveNoSpace 2
    set shellCompDirectiveNoFileComp 4
    set shellCompDirectiveFilterFileExt 8
    set shellCompDirectiveFilterDirs 16

    if test -z "$directive"
        set directive 0
    end

    set compErr (math (math --scale 0 $directive / $shellCompDirectiveError) % 2)
    if test $compErr -eq 1
        __podman_debug "Received error directive: aborting."
        # Might as well do file completion, in case it helps
        return 1
    end

    set filefilter (math (math --scale 0 $directive / $shellCompDirectiveFilterFileExt) % 2)
    set dirfilter (math (math --scale 0 $directive / $shellCompDirectiveFilterDirs) % 2)
    if test $filefilter -eq 1; or test $dirfilter -eq 1
        __podman_debug "File extension filtering or directory filtering not supported"
        # Do full file completion instead
        return 1
    end

    set nospace (math (math --scale 0 $directive / $shellCompDirectiveNoSpace) % 2)
    set nofiles (math (math --scale 0 $directive / $shellCompDirectiveNoFileComp) % 2)

    __podman_debug "nospace: $nospace, nofiles: $nofiles"

    # If we want to prevent a space, or if file completion is NOT disabled,
    # we need to count the number of valid completions.
    # To do so, we will filter on prefix as the completions we have received
    # may not already be filtered so as to allow fish to match on different
    # criteria than the prefix.
    if test $nospace -ne 0; or test $nofiles -eq 0
        set prefix (commandline -t)
        __podman_debug "prefix: $prefix"

        set completions
        for comp in $__podman_comp_results
            if test (string match -e -r -- "^$prefix" "$comp")
                set -a completions $comp
            end
        end
        set --global __podman_comp_results $completions
        __podman_debug "Filtered completions are: $__podman_comp_results"

        # Important not to quote the variable for count to work
        set numComps (count $__podman_comp_results)
        __podman_debug "numComps: $numComps"

        if test $numComps -eq 1; and test $nospace -ne 0
            # To support the "nospace" directive we trick the shell
            # by outputting an extra, longer completion.
            # We must first split on \t to get rid of the descriptions because
            # the extra character we add to the fake second completion must be
            # before the description.  We don't need descriptions anyway since
            # there is only a single real completion which the shell will expand
            # immediately.
            __podman_debug "Adding second completion to perform nospace directive"
            set split (string split --max 1 \t $__podman_comp_results[1])
            set --global __podman_comp_results $split[1] $split[1].
            __podman_debug "Completions are now: $__podman_comp_results"
        end

        if test $numComps -eq 0; and test $nofiles -eq 0
            # To be consistent with bash and zsh, we only trigger file
            # completion when there are no other completions
            __podman_debug "Requesting file completion"
            return 1
        end
    end

    return 0
end

# Since Fish completions are only loaded once the user triggers them, we trigger them ourselves
# so we can properly delete any completions provided by another script.
# The space after the program name is essential to trigger completion for the program
# and not completion of the program name itself.
complete --do-complete "podman " > /dev/null 2>&1
# Using '> /dev/null 2>&1' since '&>' is not supported in older versions of fish.

# Remove any pre-existing completions for the program since we will be handling all of them.
complete -c podman -e

# The call to __podman_prepare_completions will setup __podman_comp_results
# which provides the program's completion choices.
complete -c podman -n '__podman_prepare_completions' -f -a '$__podman_comp_results'


# This file is generated with "podman completion"; see: podman-completion(1)
