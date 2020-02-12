# #!/usr/bin/env bash
# #
# # This library defines the trap handlers for the ERR and EXIT signals. Any new handler for these signals
# # must be added to these handlers and activated by the environment variable mechanism that the rest use.
# # These functions ensure that no handler can ever alter the exit code that was emitted by a command
# # in a test script.

# os::util::trap::init_err initializes the privileged handler for the ERR signal if it hasn't
# been registered already. This will overwrite any other handlers registered on the signal.
#
# Globals:
#  None
# Arguments:
#  None
# Returns:
#  None
function os::util::trap::init_err() {
    if ! trap -p ERR | grep -q 'os::util::trap::err_handler'; then
        trap 'os::util::trap::err_handler;' ERR
    fi
}
readonly -f os::util::trap::init_err

# os::util::trap::err_handler is the handler for the ERR signal.
#
# Globals:
#  - OS_TRAP_DEBUG
#  - OS_USE_STACKTRACE
# Arguments:
#  None
# Returns:
#  - returns original return code, allows privileged handler to exit if necessary
function os::util::trap::err_handler() {
    local -r return_code=$?
    local -r last_command="${BASH_COMMAND}"

    if set +o | grep -q '\-o errexit'; then
        local -r errexit_set=true
    fi

    if [[ "${OS_TRAP_DEBUG:-}" = "true" ]]; then
        echo "[DEBUG] Error handler executing with return code \`${return_code}\`, last command \`${last_command}\`, and errexit set \`${errexit_set:-}\`"
    fi

    if [[ "${OS_USE_STACKTRACE:-}" = "true" ]]; then
        # the OpenShift stacktrace function is treated as a privileged handler for this signal
        # and is therefore allowed to run outside of a subshell in order to allow it to `exit`
        # if necessary
        os::log::stacktrace::print "${return_code}" "${last_command}" "${errexit_set:-}"
    fi

    return "${return_code}"
}
readonly -f os::util::trap::err_handler