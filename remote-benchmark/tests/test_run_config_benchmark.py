import os
import subprocess
from pathlib import Path

ROOT = Path(__file__).parents[1]
SCRIPT = ROOT / "scripts" / "devnet-local" / "run-config-benchmark.sh"


def test_script_runs_with_posix_sh(tmp_path):
    fake_bin = tmp_path / "bin"
    fake_bin.mkdir()
    command_log = tmp_path / "commands.log"
    fake_poetry = fake_bin / "poetry"
    fake_poetry.write_text("""#!/bin/sh
echo "$*" >> "$COMMAND_LOG"
if [ "$1" = run ] && [ "$2" = python ] && [ "$3" = - ]; then
  printf '10\\tsimple-transfer\\t1\\n'
elif [ "$1" = run ] && [ "$2" = remote-benchmark ] && [ "$3" = bench ]; then
  printf 'peak_tps 100.0\\noverall_tps 95.0\\ntotal_txs 10\\n'
fi
""")
    fake_poetry.chmod(0o755)
    report_path = tmp_path / "report.html"
    env = {
        **os.environ,
        "COMMAND_LOG": str(command_log),
        "PATH": f"{fake_bin}{os.pathsep}{os.environ['PATH']}",
    }

    result = subprocess.run(
        [
            "/bin/dash",
            str(SCRIPT),
            "--config",
            str(ROOT / "sample-config-anvil.yaml"),
            "--skip-fund",
            "--skip-check",
            "--output",
            str(report_path),
        ],
        cwd=ROOT,
        env=env,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 0, result.stderr
    commands = command_log.read_text()
    assert "remote-benchmark bench" in commands
    bench_command = next(
        line for line in commands.splitlines() if "remote-benchmark bench" in line
    )
    assert "--nonce" not in bench_command
    assert "python -m remote_benchmark.report" in commands
