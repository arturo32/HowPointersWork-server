# Run the Valgrind-based C/C++ backend for OPT and produce JSON to
# stdout for piping to a web app, properly handling errors and stuff
#
# Original code (c) Philip Guo, licensed under the MIT license
# Created 2016-05-09 by Philip Guo
# Modified 2018-04-28 by Kashif Nazir
# Modified 2024-11-12 by Arturo Fonseca

import json
import os
import re
import sys

from urllib.parse import parse_qs
from subprocess import Popen, PIPE


def pluck(d, *args):
    return (d[arg] for arg in args)


def parse_request(env):
    return parse_qs(env['QUERY_STRING'])


def preprocess_code(code):
    return "#define union struct\n" + code


def get_request_param(key, request_info):
    return request_info[key][0]


def setup_options():
    opts = {
        'VALGRIND_MSG_RE': re.compile('==\d+== (.*)$'),
        'PROGRAM_DIR': '/tmp/user_code',
        'LIB_DIR': '/tmp/parser', #/var/spp/lib
        'USER_PROGRAM': 'usercode.c',
        'LANG': sys.argv[1],
        'INCLUDE': '-I/var/spp/include', # TODO: update this
        'PRETTY_DUMP': False
    }
    if opts['LANG'] == 'c':
        opts['CC'] = 'gcc'
        opts['DIALECT'] = '-std=c11'
        opts['FN'] = 'usercode.c'
    elif opts['LANG'] == 'c++':
        opts['CC'] = 'g++'
        opts['DIALECT'] = '-std=c++11'
        opts['FN'] = 'usercode.cpp'
    opts.update({
        'F_PATH': os.path.join(opts['PROGRAM_DIR'], opts['FN']),
        'VGTRACE_PATH': os.path.join(opts['PROGRAM_DIR'], 'usercode.vgtrace'),
        'EXE_PATH': os.path.join(opts['PROGRAM_DIR'], 'usercode')
    })
    return opts


# Creates file usercode.c and writes all its content
# There's no need, Tork already does this
# There's also no need to clean everything, all the container will be erased
# def prep_dir(opts):
#     with open(opts['F_PATH'], 'w') as f:
#         f.write(opts['USER_PROGRAM'])



# Compile code. What's in gcc_stout, gcc_stderr, p.returncode?
def compile_c(opts):
    CC, DIALECT, EXE_PATH, F_PATH = pluck(opts, 'CC', 'DIALECT', 'EXE_PATH', 'F_PATH')
    p = Popen(
        [CC, '-ggdb', '-O0', '-fno-omit-frame-pointer', '-o', EXE_PATH, F_PATH],
        stdout=PIPE,
        stderr=PIPE
    )
    (gcc_stdout, gcc_stderr) = p.communicate() # gcc_stout specfic errors? gcc_stderr compilation errors
    gcc_retcode = p.returncode # p.returncode is the exit code, normally 0
    return gcc_retcode, gcc_stdout, gcc_stderr


def check_for_valgrind_errors(opts, valgrind_stderr):
    error_lines = []
    in_error_msg = False
    for line in valgrind_stderr.splitlines():
        m = opts['VALGRIND_MSG_RE'].match(line)
        if m:
            msg = m.group(1).rstrip()
            if 'Process terminating' in msg:
                in_error_msg = True
            if in_error_msg and not msg:
                in_error_msg = False
            if in_error_msg:
                error_lines.append(msg)
    return '\n'.join(error_lines) if error_lines else None


def run_valgrind(opts):
    VALGRIND_EXE = os.path.join(opts['LIB_DIR'], 'valgrind-3.11.0/inst/bin/valgrind')
    valgrind_p = Popen(
        ['stdbuf', '-o0',  # VERY IMPORTANT to disable stdout buffering so that stdout is traced properly
         VALGRIND_EXE,
         '--tool=memcheck',
         '--source-filename=' + opts['FN'],
         '--trace-filename=' + opts['VGTRACE_PATH'],
         opts['EXE_PATH']
         ],
        stdout=PIPE,
        stderr=PIPE
    )
    (valgrind_stdout, valgrind_stderr) = valgrind_p.communicate()
    valgrind_retcode = valgrind_p.returncode
    valgrind_out = '\n'.join(['=== Valgrind stdout ===', valgrind_stdout.decode(), '=== Valgrind stderr ===', valgrind_stderr.decode()])
    # print(valgrind_out)
    end_of_trace_error_msg = check_for_valgrind_errors(opts, str(valgrind_stderr)) if valgrind_retcode != 0 else None
    return valgrind_out, end_of_trace_error_msg


def get_opt_trace_from_vg_trace(opts, end_of_trace_error_msg):
    POSTPROCESS_EXE = os.path.join(opts['LIB_DIR'], 'vg_to_opt_trace.py')
    args = ['python3', POSTPROCESS_EXE, '--prettydump' if opts['PRETTY_DUMP'] else '--jsondump']
    if end_of_trace_error_msg:
        args += ['--end-of-trace-error-msg', end_of_trace_error_msg]
    args.append(opts['F_PATH'])
    postprocess_p = Popen(args, stdout=PIPE, stderr=PIPE)
    (postprocess_stdout, postprocess_stderr) = postprocess_p.communicate()
    postprocess_stderr = '\n'.join(['=== postprocess stderr ===', postprocess_stderr.decode(), '==='])
    return postprocess_stdout, postprocess_stderr


def generate_trace(opts, gcc_stderr):
    gcc_stderr = '\n'.join(['=== gcc stderr ===', gcc_stderr, '==='])
    (valgrind_out, end_of_trace_error_msg) = run_valgrind(opts)
    (postprocess_stdout, postprocess_stderr) = get_opt_trace_from_vg_trace(opts, valgrind_out)
    std_err = '\n'.join([gcc_stderr, valgrind_out, postprocess_stderr])
    return std_err, postprocess_stdout


def handle_gcc_error(opts, gcc_stderr):
    stderr = '\n'.join(['=== gcc stderr ===', gcc_stderr, '==='])

    exception_msg = 'unknown compiler error'
    lineno = None
    column = None

    # just report the FIRST line where you can detect a line and column
    # number of the error.
    for line in gcc_stderr.splitlines():
        # can be 'fatal error:' or 'error:' or probably other stuff too.
        m = re.search(opts['FN'] + ':(\d+):(\d+):.+?(error:.*$)', line)
        if m:
            lineno = int(m.group(1))
            column = int(m.group(2))
            exception_msg = m.group(3).strip()
            break

        # handle custom-defined errors from include path
        if '#error' in line:
            exception_msg = line.split('#error')[-1].strip()
            break

        # linker errors are usually 'undefined ' something
        # (this code is VERY brittle)
        if 'undefined ' in line:
            parts = line.split(':')
            exception_msg = parts[-1].strip()
            # match something like
            # /home/pgbovine/opt-cpp-backend/./usercode.c:2: undefined reference to `asdf'
            if opts['FN'] in parts[0]:
                try:
                    lineno = int(parts[1])
                except:
                    pass
            break

    ret = {
            'code': opts['USER_PROGRAM'],
            'trace': [
                {
                    'event': 'uncaught_exception',
                    'exception_msg': exception_msg,
                    'line': lineno
                }
            ]
        }

    return stderr, json.dumps(ret)


# def cleanup(opts):
#     shutil.rmtree(opts['PROGRAM_DIR'])


def application():
    opts = setup_options()
    # (gcc_retcode, gcc_stdout, gcc_stderr) = compile_c(opts)
    (stderr, stdout) = generate_trace(opts, "")
    # if gcc_retcode == 0 else handle_gcc_error(opts, gcc_stderr)
    # cleanup(opts)
    # TODO: Figure out how to handle stderr
    print(stdout.decode())
    # print('-------------')
    # print(stderr)
    return [stdout]



if __name__ == "__main__":
    application()
