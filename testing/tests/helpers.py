import os
import subprocess
import time
from collections import namedtuple
from contextlib import contextmanager

import pytest
import requests

SCRIPT_DIR = os.path.dirname(os.path.abspath(__file__))
LOAD_GEN_DIR = os.path.abspath(os.path.join(SCRIPT_DIR, "../../"))

RunResponse = namedtuple("RunResponse", ["returncode", "stdout", "stderr", "error_msg"])


@contextmanager
def run_server(with_errors=False):
    # disable warnings for not verifying the certs
    requests.packages.urllib3.disable_warnings()

    server_path = os.path.abspath(os.path.join(LOAD_GEN_DIR, "test-server"))

    cmd = [server_path]
    if with_errors:
        cmd.append("-with-errors")
    p = subprocess.Popen(
        cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=LOAD_GEN_DIR
    )

    try:
        ok = False
        started = time.time()
        while time.time() - started < 10:
            try:
                r = requests.get("https://127.0.0.1:8080/ready", verify=False)
                if r.status_code == 200:
                    ok = True
                    break
            except Exception as e:
                print(e)
            time.sleep(0.3)

        if not ok:
            raise Exception("failed to start test server")

        # A small delay is needed to check if the server has exited.
        # This could happen if someone was running it manually.
        time.sleep(0.2)
        if p.poll():
            o, e = p.communicate()
            o = o.decode("utf-8") if o else ""
            e = e.decode("utf-8") if e else ""
            raise Exception(f"server has exited\nstdout:\n{o}\nstderr:\n{e}")

        yield p
    finally:
        p.kill()
        p.communicate()


@pytest.fixture(scope="module")
def server():
    with run_server() as s:
        yield s


def run(cmd, cwd=None):
    p = subprocess.Popen(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, cwd=cwd)
    o, e = p.communicate()

    o = o.decode("utf-8")
    e = e.decode("utf-8")

    error_msg = f"stdout:\n{o}\n\nstderr:\n{e}\n"

    return RunResponse(p.returncode, o, e, error_msg)
