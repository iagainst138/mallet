import os

import helpers


def test_latency():
    cmd = [
        os.path.join(helpers.LOAD_GEN_DIR, "latency-tool"),
        "-H",
        "https://localhost:8080",
        "-k",
        "-i",
        "10",
        "-c",
        "generic_http/sample.yaml",
        "-redial",
        "3",
    ]

    with helpers.run_server():
        r = helpers.run(cmd, cwd=helpers.LOAD_GEN_DIR)
        assert r.returncode == 0, r.error_msg
        #assert "200" in r.stdout
