import os
import subprocess


class ChainCommand:
    def __init__(self, cmd):
        self.cmd = cmd

    def raw(self, *args, stdin=None, stderr=subprocess.STDOUT, **kwargs):
        "execute the command"
        args = " ".join(build_cli_args_safe(*args, **kwargs))
        stdout = interact(
            f"{self.cmd} {args}", input=stdin, stderr=stderr, env=os.environ
        )

        # filter out "<jemalloc>:" warning messages
        stdout = b"\n".join(
            line for line in stdout.splitlines() if not line.startswith(b"<jemalloc>:")
        )

        return stdout

    def __call__(self, *args, **kwargs):
        "execute the command and clean the output"
        return self.raw(*args, **kwargs).decode().strip()


def interact(cmd, ignore_error=False, input=None, stderr=subprocess.STDOUT, **kwargs):
    proc = subprocess.Popen(
        cmd,
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
        stderr=stderr,
        shell=True,
        **kwargs,
    )
    # begin = time.perf_counter()
    (stdout, _) = proc.communicate(input=input)
    # print('[%.02f] %s' % (time.perf_counter() - begin, cmd))
    if not ignore_error:
        assert proc.returncode == 0, f'{stdout.decode("utf-8")} ({cmd})'
    return stdout


def build_cli_args_safe(*args, **kwargs):
    args = [safe_cli_string(arg) for arg in args if arg]
    for k, v in kwargs.items():
        if v is None:
            continue
        args.append("--" + k.strip("_").replace("_", "-"))
        args.append(safe_cli_string(v))
    return list(map(str, args))


def safe_cli_string(s):
    'wrap string in "", used for cli argument when contains spaces'
    if len(f"{s}".split()) > 1:
        return f"'{s}'"
    return f"{s}"
