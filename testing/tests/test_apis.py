import os
import re

import pytest

import helpers

HTTP_ARGS = ["-a", "https://127.0.0.1:8080"]


@pytest.mark.parametrize(
    "service, with_errors, args",
    [
        [
            "http",
            False,
            HTTP_ARGS
            + ["-g", os.path.join(helpers.LOAD_GEN_DIR, "scripts", "generator.py")],
        ],
        [
            "http",
            False,
            HTTP_ARGS
            + [
                "-g",
                os.path.join(
                    helpers.LOAD_GEN_DIR, "scripts", "generator-with-delay.py"
                ),
            ],
        ],
        [
            "http",
            False,
            HTTP_ARGS
            + [
                "-g",
                os.path.join(helpers.LOAD_GEN_DIR, "scripts", "generator-one-shot.py"),
            ],
        ],
        [
            "http",
            True,
            HTTP_ARGS
            + ["-g", os.path.join(helpers.LOAD_GEN_DIR, "scripts", "generator.py")],
        ],
    ],
)
def test_queries(service, with_errors, args):
    chart_path = "/tmp/test-chart.html"
    logfile_path = "/tmp/test-logfile.log"

    for p in [chart_path, logfile_path]:
        if os.path.exists(p):
            os.remove(p)

    cmd = [
        os.path.join(helpers.LOAD_GEN_DIR, "out/mallet.out"),
        service,
        "-k",
        "-n",
        "500",
        "-d",
        "2s",
        "-r",
        "1s",
        "-w",
        "1s",
        "-e",
        "100",
        "-chart",
        chart_path,
        "-logfile",
        logfile_path,
    ]
    cmd += args

    # this regex is to find the table in the stdout
    regex = re.compile(".*STARTED.*MIN.*MAX.*P95.*ERRORS.*ROLLED.*UP.*COUNT")

    with helpers.run_server(with_errors=with_errors):
        r = helpers.run(cmd)
        if with_errors:
            assert r.returncode != 0, r.error_msg
        else:
            assert r.returncode == 0, r.error_msg
            assert regex.findall(r.stdout), r.error_msg

    assert os.path.exists(chart_path)
    assert os.path.exists(logfile_path)

    with open(logfile_path) as f:
        log_lines = f.readlines()

    if with_errors:
        assert len(log_lines) > 0
    else:
        assert len(log_lines) == 0
