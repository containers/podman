'''Plugin to override the default output logic.'''

# upstream: https://gist.github.com/cliffano/9868180

# This program is free software: you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation, either version 3 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.


# For some reason this has to be done
import imp
import os

ANSIBLE_PATH = imp.find_module('ansible')[1]
DEFAULT_PATH = os.path.join(ANSIBLE_PATH, 'plugins/callback/default.py')
DEFAULT_MODULE = imp.load_source(
    'ansible.plugins.callback.default',
    DEFAULT_PATH
)

try:
    from ansible.plugins.callback import CallbackBase
    BASECLASS = CallbackBase
except ImportError: # < ansible 2.1
    BASECLASS = DEFAULT_MODULE.CallbackModule


class CallbackModule(DEFAULT_MODULE.CallbackModule):  # pylint: disable=too-few-public-methods,no-init
    '''
    Override for the default callback module.

    Render std err/out outside of the rest of the result which it prints with
    indentation.
    '''
    CALLBACK_VERSION = 2.0
    CALLBACK_TYPE = 'stdout'
    CALLBACK_NAME = 'default'

    def __init__(self, *args, **kwargs):
        # pylint: disable=non-parent-init-called
        BASECLASS.__init__(self, *args, **kwargs)
        self.failed_task = []
        self.result_file = os.environ.get('AHT_RESULT_FILE')

    def _dump_results(self, result):
        '''Return the text to output for a result.'''
        result['_ansible_verbose_always'] = True

        save = {}
        for key in ['stdout', 'stdout_lines', 'stderr', 'stderr_lines', 'msg']:
            if key in result:
                save[key] = result.pop(key)

        output = BASECLASS._dump_results(self, result)  # pylint: disable=protected-access

        for key in ['stdout', 'stderr', 'msg']:
            if key in save and save[key]:
                output += '\n\n%s:\n---\n%s\n---' % (key.upper(), save[key])

        for key, value in save.items():
            result[key] = value

        return output

    def v2_runner_on_unreachable(self, result):
        self.failed_task = result

        if self._play.strategy == 'free' and self._last_task_banner != result._task._uuid:
            self._print_task_banner(result._task)

        delegated_vars = result._result.get('_ansible_delegated_vars', None)
        if delegated_vars:
            self._display.display("fatal: [%s -> %s]: UNREACHABLE! => %s" % (result._host.get_name(), delegated_vars['ansible_host'], self._dump_results(result._result)), color=C.COLOR_UNREACHABLE)
        else:
            self._display.display("fatal: [%s]: UNREACHABLE! => %s" % (result._host.get_name(), self._dump_results(result._result)), color=C.COLOR_UNREACHABLE)

    def v2_runner_on_failed(self,result, ignore_errors=False):
        if ignore_errors is not True:
            # Sets environment variable for test failures for use in playboks.
            # Handlers tasks can conditionalize themselves using this variable
            # to run only on failure.
            os.environ["AHT_FAILURE"] = "1"

            # Save last failure
            self.failed_task = result

        if self._play.strategy == 'free' and self._last_task_banner != result._task._uuid:
            self._print_task_banner(result._task)

        delegated_vars = result._result.get('_ansible_delegated_vars', None)
        if 'exception' in result._result:
            if self._display.verbosity < 3:
                # extract just the actual error message from the exception text
                error = result._result['exception'].strip().split('\n')[-1]
                msg = "An exception occurred during task execution. To see the full traceback, use -vvv. The error was: %s" % error
            else:
                msg = "An exception occurred during task execution. The full traceback is:\n" + result._result['exception']

            self._display.display(msg, color=C.COLOR_ERROR)

        if result._task.loop and 'results' in result._result:
            self._process_items(result)

        else:
            if delegated_vars:
                self._display.display("fatal: [%s -> %s]: FAILED! => %s" % (result._host.get_name(), delegated_vars['ansible_host'], self._dump_results(result._result)), color=C.COLOR_ERROR)
            else:
                self._display.display("fatal: [%s]: FAILED! => %s" % (result._host.get_name(), self._dump_results(result._result)), color=C.COLOR_ERROR)

        if ignore_errors:
            self._display.display("...ignoring", color=C.COLOR_SKIP)

    def v2_playbook_on_stats(self, stats):
        self._display.banner("PLAY RECAP")

        hosts = sorted(stats.processed.keys())
        for h in hosts:
            t = stats.summarize(h)

            self._display.display(u"%s : %s %s %s %s" % (
                hostcolor(h, t),
                colorize(u'ok', t['ok'], C.COLOR_OK),
                colorize(u'changed', t['changed'], C.COLOR_CHANGED),
                colorize(u'unreachable', t['unreachable'], C.COLOR_UNREACHABLE),
                colorize(u'failed', t['failures'], C.COLOR_ERROR)),
                screen_only=True
            )

            self._display.display(u"%s : %s %s %s %s" % (
                hostcolor(h, t, False),
                colorize(u'ok', t['ok'], None),
                colorize(u'changed', t['changed'], None),
                colorize(u'unreachable', t['unreachable'], None),
                colorize(u'failed', t['failures'], None)),
                log_only=True
            )

        self._display.display("", screen_only=True)
        # Save result to file if environment variable exists
        if self.result_file is not None:
            if self.failed_task:
                with open(self.result_file, 'w') as f:
                    f.write("PLAY: %s\n%s\n%s" % (self._play, \
                                                  self.failed_task._task, \
                                                  self._dump_results(self.failed_task._result)))
            else:
                open(self.result_file, 'w').close()
