#!/usr/bin/env python3

import json
import time

i = 0
while True:
    r = {
        "method": "GET",
        "query": "/test/",
        "body": "some data",
    }

    if i == 1:
        r["method"] = "POST"

    print(json.dumps(r))
    i += 1

    time.sleep(3)
